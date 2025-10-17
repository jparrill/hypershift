package syncglobalpullsecret

// sync-global-pullsecret syncs the pull secret from the user provided pull secret in DataPlane and appends it to the HostedCluster PullSecret to be deployed in the nodes of the HostedCluster.
// It uses CRIO's global_global_auth_file configuration strategy instead of directly modifying kubelet config to avoid conflicts with machineConfigOperator during upgrades.

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/coreos/go-systemd/dbus"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/openshift/hypershift/sync-global-pullsecret/manifests"
)

// syncGlobalPullSecretOptions contains the configuration options for the sync-global-pullsecret command
type syncGlobalPullSecretOptions struct {
	// No configuration options needed - CRIO configuration paths are fixed
}

//go:generate ../hack/tools/bin/mockgen -destination=sync-global-pullsecret_mock.go -package=syncglobalpullsecret . dbusConn
type dbusConn interface {
	RestartUnit(name string, mode string, ch chan<- string) (int, error)
	Close()
}

// GlobalPullSecretSyncer handles the synchronization of pull secrets using CRIO global_auth_file configuration.
// This approach configures CRIO to use custom authentication files instead of modifying kubelet config,
// which avoids conflicts with machineConfigOperator during cluster upgrades.
type GlobalPullSecretSyncer struct {
	// crioGlobalPullSecretConfigPath is the path to the CRIO drop-in config file that sets global_auth_file
	crioGlobalPullSecretConfigPath string
	// globalPullSecretPath is the path where the merged pull secret is written for CRIO to use
	globalPullSecretPath string
	// hcpOriginalPullSecretFilePath is the path to the original pull secret from HostedControlPlane
	hcpOriginalPullSecretFilePath string
	// hcpGlobalPullSecretFilePath is the path to the global pull secret from HostedControlPlane
	hcpGlobalPullSecretFilePath string
	// signalDBUSToRestartCrioFunc is the function used to restart CRIO when configuration changes
	signalDBUSToRestartCrioFunc func() error
	log                         logr.Logger
}

const (
	dbusRestartUnitMode = "replace"
	kubeletServiceUnit  = "kubelet.service"
	crioServiceUnit     = "crio.service"

	// Default mounted secret file paths from the DaemonSet volumes
	defaultHcpOriginalPullSecretFilePath = "/etc/original-pull-secret/.dockerconfigjson"
	defaultHcpGlobalPullSecretFilePath   = "/etc/global-pull-secret/.dockerconfigjson"

	// CRIO configuration strategy paths:
	// - CRIO drop-in config file that sets global_auth_file directive
	defaultCrioGlobalPullSecretConfigPath = "/etc/crio/crio.conf.d/99-global-pull-secret.conf"
	// - Path where the merged pull secret is written for CRIO to use
	defaultGlobalPullSecretPath = "/etc/hypershift/global-pull-secret.json"

	tickerPace = 30 * time.Second

	// systemd job completion state as documented in go-systemd/dbus
	systemdJobDone = "done" // Job completed successfully
)

var (
	// writeFileFunc is a variable that holds the function used to write files.
	// This allows tests to inject custom write functions for testing rollback scenarios.
	writeFileFunc = writeAtomic

	// readFileFunc is a variable that holds the function used to read files.
	// This allows tests to inject custom read functions for testing.
	readFileFunc = os.ReadFile
)

// NewRunCommand creates a new cobra.Command for the sync-global-pullsecret command
func NewRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync-global-pullsecret",
		Short: "Syncs a mixture between the user original pull secret in DataPlane and the HostedCluster PullSecret to be deployed in the nodes of the HostedCluster",
		Long:  `Syncs a mixture between the user original pull secret in DataPlane and the HostedCluster PullSecret to be deployed in the nodes of the HostedCluster. The resulting pull secret is deployed in a DaemonSet in the DataPlane that configures CRIO to use the appropriate authentication via global_auth_file configuration. This approach avoids conflicts with machineConfigOperator during upgrades. If there are conflicting entries in the resulting global pull secret, the original pull secret entries will prevail to ensure the well functioning of the nodes.`,
	}

	opts := syncGlobalPullSecretOptions{}
	cmd.Run = func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle SIGINT and SIGTERM
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigChan
			cancel()
		}()

		if err := opts.run(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	return cmd
}

// run executes the main logic of the sync-global-pullsecret command
func (o *syncGlobalPullSecretOptions) run(ctx context.Context) error {
	// Setup logger using zap with logr interface
	config := zap.NewProductionConfig()
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	zapLogger, err := config.Build()
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	logger := zapr.NewLogger(zapLogger)

	// Create syncer
	syncer := &GlobalPullSecretSyncer{
		crioGlobalPullSecretConfigPath: defaultCrioGlobalPullSecretConfigPath,
		globalPullSecretPath:           defaultGlobalPullSecretPath,
		hcpOriginalPullSecretFilePath:  defaultHcpOriginalPullSecretFilePath,
		hcpGlobalPullSecretFilePath:    defaultHcpGlobalPullSecretFilePath,
		signalDBUSToRestartCrioFunc:    signalDBUSToRestartCrio,
		log:                            logger,
	}

	// Start the sync loop
	return syncer.runSyncLoop(ctx)
}

// runSyncLoop runs the main synchronization loop with backoff
func (s *GlobalPullSecretSyncer) runSyncLoop(ctx context.Context) error {
	s.log.Info("Starting global pull secret sync loop")

	// Initial sync
	if err := s.globalPullSecret(); err != nil {
		s.log.Error(err, "Initial sync failed")
	}

	// Sync loop with backoff
	ticker := time.NewTicker(tickerPace)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.log.Info("Context canceled, stopping sync loop")
			return nil
		case <-ticker.C:
			if err := s.globalPullSecret(); err != nil {
				s.log.Error(err, "Sync failed")
				// Continue the loop even if sync fails
			}
		}
	}
}

// globalPullSecret handles the update logic for the GlobalPullSecret
// Main loop
func (s *GlobalPullSecretSyncer) globalPullSecret() error {
	s.log.Info("Checking global pull secret")

	// Step 1: Decide which pull secret to use and whether we need CRIO custom config
	useGlobalPullSecret, finalPullSecretBytes, err := s.decidePullSecretStrategy()
	if err != nil {
		return fmt.Errorf("failed to decide pull secret strategy: %w", err)
	}

	// Step 2: Configure CRIO based on the decision
	if err := s.configureCRIO(useGlobalPullSecret, finalPullSecretBytes); err != nil {
		return fmt.Errorf("failed to configure CRIO: %w", err)
	}

	// Configuration completed successfully - CRIO will handle authentication via global_auth_file
	// No verification needed since we're not modifying kubelet config directly
	return nil
}

// decidePullSecretStrategy determines which pull secret to use and whether CRIO needs custom config
func (s *GlobalPullSecretSyncer) decidePullSecretStrategy() (useGlobalPullSecret bool, finalPullSecret []byte, err error) {
	// Always read the original pull secret (we'll need it regardless)
	originalPullSecretBytes, err := readPullSecretFromFile(s.hcpOriginalPullSecretFilePath)
	if err != nil {
		return false, nil, fmt.Errorf("failed to read original pull secret: %w", err)
	}

	// Try to read the global pull secret
	globalPullSecretBytes, err := readPullSecretFromFile(s.hcpGlobalPullSecretFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Global pull secret file doesn't exist -> use original
			s.log.Info("Global pull secret file not found, using original pull secret")
			return false, originalPullSecretBytes, nil
		}
		// Other error reading global pull secret
		return false, nil, fmt.Errorf("failed to read global pull secret file: %w", err)
	}

	// Global pull secret file exists, check if it has content
	if len(globalPullSecretBytes) == 0 {
		s.log.Info("Global pull secret file is empty, using original pull secret")
		return false, originalPullSecretBytes, nil
	}

	// Validate global pull secret format
	if err := validateDockerConfigJSON(globalPullSecretBytes); err != nil {
		s.log.Error(err, "Global pull secret has invalid format, falling back to original")
		return false, originalPullSecretBytes, nil
	}

	// Global pull secret exists and is valid -> use it
	s.log.Info("Global pull secret found and valid, using global pull secret")
	return true, globalPullSecretBytes, nil
}

// configureCRIO configures CRIO based on whether we're using global pull secret or not.
// This function implements the CRIO global_auth_file strategy that avoids conflicts with machineConfigOperator:
// - When useGlobalPullSecret=true: Creates CRIO drop-in config pointing to custom auth file
// - When useGlobalPullSecret=false: Removes CRIO config to use default kubelet auth
func (s *GlobalPullSecretSyncer) configureCRIO(useGlobalPullSecret bool, pullSecretBytes []byte) error {
	if useGlobalPullSecret {
		s.log.Info("Configuring CRIO to use global pull secret from custom path")
		return s.enableGlobalPullSecretCRIOConfig(pullSecretBytes)
	} else {
		s.log.Info("Configuring CRIO to use original pull secret (cleaning up custom config)")
		return s.disableGlobalPullSecretCRIOConfig()
	}
}

// enableGlobalPullSecretCRIOConfig configures CRIO to use the global pull secret.
// Creates /etc/crio/crio.conf.d/99-global-pull-secret.conf with global_auth_file directive
// pointing to /etc/hypershift/global-pull-secret.json. Only restarts CRIO if config changed.
func (s *GlobalPullSecretSyncer) enableGlobalPullSecretCRIOConfig(pullSecretBytes []byte) error {
	var needsCRIORestart bool

	// Check if global pull secret content has changed
	existingPullSecret, err := readFileFunc(s.globalPullSecretPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing global pull secret: %w", err)
	}

	if !comparePullSecretBytes(existingPullSecret, pullSecretBytes) {
		// Write the global pull secret to the custom path
		if err := writeFileFunc(s.globalPullSecretPath, pullSecretBytes, 0600); err != nil {
			return fmt.Errorf("failed to write global pull secret to %s: %w", s.globalPullSecretPath, err)
		}
		s.log.Info("Global pull secret written", "file", s.globalPullSecretPath)
	}

	// Generate CRIO configuration to point to the custom path
	crioConfig := manifests.CRIOGlobalAuthFileConfig(s.globalPullSecretPath)

	// Check if CRIO config has changed
	existingCRIOConfig, err := readFileFunc(s.crioGlobalPullSecretConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing CRIO config: %w", err)
	}

	if string(existingCRIOConfig) != crioConfig {
		needsCRIORestart = true

		// Ensure the CRIO config directory exists
		crioConfigDir := filepath.Dir(s.crioGlobalPullSecretConfigPath)
		if _, err := os.Stat(crioConfigDir); os.IsNotExist(err) {
			return fmt.Errorf("CRIO config directory %s does not exist", crioConfigDir)
		} else if err != nil {
			return fmt.Errorf("failed to check CRIO config directory %s: %w", crioConfigDir, err)
		}

		// Write CRIO configuration
		if err := writeFileFunc(s.crioGlobalPullSecretConfigPath, []byte(crioConfig), 0644); err != nil {
			return fmt.Errorf("failed to write CRIO config to %s: %w", s.crioGlobalPullSecretConfigPath, err)
		}
		s.log.Info("CRIO configuration updated", "file", s.crioGlobalPullSecretConfigPath)
	} else {
		s.log.Info("CRIO configuration unchanged, no restart needed")
	}

	// Restart CRIO only if configuration changed
	if needsCRIORestart {
		s.log.Info("CRIO configuration changed, restarting CRIO")
		return s.signalDBUSToRestartCrioFunc()
	}

	return nil
}

// disableGlobalPullSecretCRIOConfig removes CRIO configuration for global pull secret.
// Removes /etc/crio/crio.conf.d/99-global-pull-secret.conf and /etc/hypershift/global-pull-secret.json
// so CRIO falls back to using default kubelet authentication. Only restarts CRIO if config actually existed.
func (s *GlobalPullSecretSyncer) disableGlobalPullSecretCRIOConfig() error {
	var needsCRIORestart bool

	// Check if CRIO configuration file exists
	_, err := os.Stat(s.crioGlobalPullSecretConfigPath)
	if err == nil {
		// File exists, remove it
		if err := os.Remove(s.crioGlobalPullSecretConfigPath); err != nil {
			return fmt.Errorf("failed to remove CRIO config file %s: %w", s.crioGlobalPullSecretConfigPath, err)
		}
		s.log.Info("CRIO configuration file removed", "file", s.crioGlobalPullSecretConfigPath)
		needsCRIORestart = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check CRIO config file %s: %w", s.crioGlobalPullSecretConfigPath, err)
	} else {
		s.log.Info("CRIO configuration file does not exist, no action needed")
	}

	// Check if global pull secret file exists, at this point it should not exist
	_, err = os.Stat(s.globalPullSecretPath)
	if err == nil {
		// File exists, remove it
		if err := os.Remove(s.globalPullSecretPath); err != nil {
			return fmt.Errorf("failed to remove global pull secret file %s: %w", s.globalPullSecretPath, err)
		}
		s.log.Info("Global pull secret file removed", "file", s.globalPullSecretPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check global pull secret file %s: %w", s.globalPullSecretPath, err)
	}

	// Restart CRIO only if configuration was actually removed
	if needsCRIORestart {
		s.log.Info("CRIO configuration was removed, restarting CRIO")
		return s.signalDBUSToRestartCrioFunc()
	} else {
		s.log.Info("CRIO configuration unchanged, no restart needed")
	}

	return nil
}

// signalDBUSToRestartCrio signals CRIO to restart by restarting the crio.service.
// This is done by sending a signal to systemd via dbus.
func signalDBUSToRestartCrio() error {
	conn, err := dbus.New()
	if err != nil {
		return fmt.Errorf("failed to connect to dbus: %w", err)
	}
	defer conn.Close()

	return restartSystemdUnit(conn, crioServiceUnit)
}
