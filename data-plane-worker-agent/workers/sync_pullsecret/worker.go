package sync_pullsecret

import (
	"context"
	"fmt"
	"os"

	cmdutil "github.com/openshift/hypershift/cmd/util"
	"github.com/openshift/hypershift/data-plane-worker-agent/workers"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SyncPullSecretWorker struct {
	workers.BaseWorker
	client client.Client
}

func New() *SyncPullSecretWorker {
	return &SyncPullSecretWorker{
		BaseWorker: workers.BaseWorker{
			NameField:        "sync-pullsecret",
			DescriptionField: "Syncs pull secrets from DataPlane to kubelet config",
		},
	}
}

// Reconcile implements the reconciliation logic for syncing pull secrets
func (s *SyncPullSecretWorker) Reconcile(ctx context.Context, task workers.Task) (bool, error) {
	logger := log.FromContext(ctx)

	// Initialize client if not already done
	if s.client == nil {
		c, err := cmdutil.GetClient()
		if err != nil {
			return true, fmt.Errorf("failed to create client: %w", err)
		}
		s.client = c
	}
	// Validate if the Config is updated via MC
	kubeletConfigPath := workers.GetStringParam(task.Parameters, "kubelet-config-json-path", "/var/lib/kubelet/config.json")
	secretName := workers.GetStringParam(task.Parameters, "global-ps-secret-name", "global-pull-secret")

	// Read existing content
	existingContent, err := os.ReadFile(kubeletConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return true, fmt.Errorf("failed to read existing file: %w", err)
	}

	// Get global pull secret
	globalPullSecret := &corev1.Secret{}
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: "kube-system", Name: secretName}, globalPullSecret); err != nil {
		return true, fmt.Errorf("failed to get global pull secret: %w", err)
	}

	globalPullSecretBytes := globalPullSecret.Data[corev1.DockerConfigJsonKey]

	// Check if update is needed
	if string(existingContent) != string(globalPullSecretBytes) {
		if err := os.WriteFile(kubeletConfigPath, globalPullSecretBytes, 0600); err != nil {
			return true, fmt.Errorf("failed to write file: %w", err)
		}
		logger.Info("Updated kubelet config with new pull secret", "path", kubeletConfigPath)

		// Requeue to verify the change was successful
		return true, nil
	}

	// No changes needed, wait for next interval
	return false, nil
}

func (s *SyncPullSecretWorker) Validate(task workers.Task) error {
	if _, ok := task.Parameters["kubelet-config-json-path"]; !ok {
		return fmt.Errorf("kubelet-config-json-path parameter is required")
	}
	return nil
}

func (s *SyncPullSecretWorker) DefaultParameters() map[string]interface{} {
	return map[string]interface{}{
		"kubelet-config-json-path": "/var/lib/kubelet/config.json",
		"global-ps-secret-name":    "global-pull-secret",
		"check-interval":           "30s",
	}
}

func init() {
	workers.Register(New())
}
