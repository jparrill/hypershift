package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift/hypershift/data-plane-worker-agent/controllers"
	"github.com/openshift/hypershift/data-plane-worker-agent/workers"
	"github.com/openshift/hypershift/data-plane-worker-agent/workers/sync_pullsecret"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type AgentConfig struct {
	EnabledWorkers []string                `json:"enabledWorkers"`
	WorkerConfigs  map[string]WorkerConfig `json:"workerConfigs"`
	LogLevel       string                  `json:"logLevel"`
}

type WorkerConfig struct {
	Enabled    bool                   `json:"enabled"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type Agent struct {
	config    *AgentConfig
	tasks     map[string]*workers.Task
	namespace string
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewAgent(config *AgentConfig) *Agent {
	ctx, cancel := context.WithCancel(context.Background())
	return &Agent{
		config:    config,
		tasks:     make(map[string]*workers.Task),
		namespace: "kube-system", // Por defecto usamos kube-system
		ctx:       ctx,
		cancel:    cancel,
	}
}

func (a *Agent) Start() error {
	logger := log.FromContext(a.ctx)
	logger.Info("Starting Data Plane Worker Agent")

	// Register available workers
	if err := a.registerWorkers(); err != nil {
		return fmt.Errorf("failed to register workers: %w", err)
	}

	// Create tasks for enabled workers
	if err := a.createTasks(); err != nil {
		return fmt.Errorf("failed to create tasks: %w", err)
	}

	// Set up scheme for the manager
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	// Create manager
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("unable to start manager: %w", err)
	}

	// Create initial ConfigMaps
	if err := a.createInitialConfigs(a.ctx, mgr.GetClient()); err != nil {
		return fmt.Errorf("failed to create initial configs: %w", err)
	}

	// Set up controllers for each worker
	for workerName := range a.tasks {
		worker, exists := workers.Get(workerName)
		if !exists {
			return fmt.Errorf("worker %s not found", workerName)
		}

		reconciler := &controllers.WorkerReconciler{
			Client:          mgr.GetClient(),
			Worker:          worker,
			ConfigNamespace: a.namespace,
		}

		if err := reconciler.SetupWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create controller for worker %s: %w", workerName, err)
		}

		logger.Info("Created controller for worker", "worker", workerName)
	}

	// Start manager
	logger.Info("Starting manager")
	if err := mgr.Start(a.ctx); err != nil {
		return fmt.Errorf("problem running manager: %w", err)
	}

	return nil
}

func (a *Agent) registerWorkers() error {
	// Register sync-pullsecret worker
	syncPullSecretWorker := sync_pullsecret.New()
	if err := workers.Register(syncPullSecretWorker); err != nil {
		return fmt.Errorf("failed to register sync-pullsecret worker: %w", err)
	}

	log.FromContext(a.ctx).Info("Workers registered successfully")
	return nil
}

func (a *Agent) createTasks() error {
	logger := log.FromContext(a.ctx)

	for _, workerName := range a.config.EnabledWorkers {
		worker, exists := workers.Get(workerName)
		if !exists {
			return fmt.Errorf("worker %s not found", workerName)
		}

		config, exists := a.config.WorkerConfigs[workerName]
		if !exists {
			config = WorkerConfig{Enabled: true}
		}

		if !config.Enabled {
			logger.Info("Worker is disabled, skipping", "worker", workerName)
			continue
		}

		// Merge default parameters with provided parameters
		parameters := worker.DefaultParameters()
		for k, v := range config.Parameters {
			parameters[k] = v
		}

		task := &workers.Task{
			Name:       workerName,
			Parameters: parameters,
			Enabled:    config.Enabled,
		}

		a.tasks[workerName] = task
		logger.Info("Created task for worker", "worker", workerName)
	}

	return nil
}

func (a *Agent) createInitialConfigs(ctx context.Context, c client.Client) error {
	for workerName, task := range a.tasks {
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-config", workerName),
				Namespace: a.namespace,
			},
		}

		configJSON, err := json.Marshal(task)
		if err != nil {
			return fmt.Errorf("failed to marshal config for worker %s: %w", workerName, err)
		}

		if err := c.Create(ctx, configMap); err != nil {
			if client.IgnoreAlreadyExists(err) != nil {
				return fmt.Errorf("failed to create ConfigMap for worker %s: %w", workerName, err)
			}
			// Si ya existe, actualizamos
			if err := c.Update(ctx, configMap); err != nil {
				return fmt.Errorf("failed to update ConfigMap for worker %s: %w", workerName, err)
			}
		}

		// Establecer los datos después de crear/actualizar
		configMap.Data = map[string]string{
			"config.json": string(configJSON),
		}
		if err := c.Update(ctx, configMap); err != nil {
			return fmt.Errorf("failed to update ConfigMap data for worker %s: %w", workerName, err)
		}
	}
	return nil
}

func (a *Agent) waitForShutdown() {
	logger := log.FromContext(a.ctx)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	logger.Info("Waiting for shutdown signal...")
	<-sigChan

	logger.Info("Shutdown signal received, stopping agent...")
	a.cancel()
}

func NewStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Data Plane Worker Agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath, _ := cmd.Flags().GetString("config")

			// Load configuration from file
			config, err := loadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			agent := NewAgent(config)
			return agent.Start()
		},
	}

	cmd.Flags().String("config", "/etc/data-plane-worker-agent/config.json", "Path to configuration file")
	return cmd
}

func loadConfig(configPath string) (*AgentConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AgentConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// Helper function to get string parameter with default
func getStringParam(params map[string]interface{}, key, defaultValue string) string {
	if value, exists := params[key]; exists {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}
