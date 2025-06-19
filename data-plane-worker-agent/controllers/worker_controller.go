package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openshift/hypershift/data-plane-worker-agent/workers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// WorkerReconciler reconciles a Worker's ConfigMap
type WorkerReconciler struct {
	client.Client
	Worker          workers.Worker
	ConfigNamespace string
}

// Reconcile handles the reconciliation loop for a worker's configuration
func (r *WorkerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("worker", r.Worker.Name())

	// Obtener el ConfigMap de configuración
	configMap := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: r.ConfigNamespace,
		Name:      fmt.Sprintf("%s-config", r.Worker.Name()),
	}, configMap); err != nil {
		logger.Error(err, "Unable to fetch ConfigMap")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Parsear la configuración del ConfigMap
	task, err := ParseWorkerConfig(configMap)
	if err != nil {
		logger.Error(err, "Invalid configuration in ConfigMap")
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	// Validar la configuración
	if err := r.Worker.Validate(task); err != nil {
		logger.Error(err, "Configuration validation failed")
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	// Ejecutar la reconciliación
	requeue, err := r.Worker.Reconcile(ctx, task)
	if err != nil {
		logger.Error(err, "Reconciliation failed")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	if requeue {
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}

	interval, _ := time.ParseDuration(workers.GetStringParam(task.Parameters, "check-interval", "30s"))
	return ctrl.Result{RequeueAfter: interval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.ConfigMap{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(obj client.Object) bool {
			// Solo procesar ConfigMaps que pertenecen a este worker
			return obj.GetName() == fmt.Sprintf("%s-config", r.Worker.Name()) &&
				obj.GetNamespace() == r.ConfigNamespace
		})).
		Complete(r)
}

// ParseWorkerConfig convierte un ConfigMap en una Task
func ParseWorkerConfig(cm *corev1.ConfigMap) (workers.Task, error) {
	var task workers.Task

	// Asumimos que tenemos un campo 'config.json' en el ConfigMap
	configJSON, exists := cm.Data["config.json"]
	if !exists {
		return task, fmt.Errorf("config.json not found in ConfigMap")
	}

	if err := json.Unmarshal([]byte(configJSON), &task); err != nil {
		return task, fmt.Errorf("failed to parse config: %w", err)
	}

	return task, nil
}
