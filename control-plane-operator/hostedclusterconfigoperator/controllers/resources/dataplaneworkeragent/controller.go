package dataplaneworkeragent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	"github.com/openshift/hypershift/control-plane-operator/controllers/hostedcontrolplane/manifests"
	"github.com/openshift/hypershift/support/upsert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	logr "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	EnableDataPlaneWorkerAgentAnnotation  = "hypershift.openshift.io/enable-data-plane-worker-agent"
	DataPlaneWorkerAgentWorkersAnnotation = "hypershift.openshift.io/data-plane-worker-agent-workers"
	DataPlaneWorkerAgentName              = "data-plane-worker-agent"
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

func ReconcileDataPlaneWorkerAgent(ctx context.Context, c crclient.Client, createOrUpdate upsert.CreateOrUpdateFN, hcp *hyperv1.HostedControlPlane, cpoImage string) error {
	// Check if the agent is enabled
	if !isDataPlaneWorkerAgentEnabled(hcp) {
		return nil
	}

	// Get enabled workers
	enabledWorkers := getEnabledWorkers(hcp)

	// Reconcile RBAC resources
	if err := reconcileWorkerAgentRBAC(ctx, c, createOrUpdate, hcp.Namespace, enabledWorkers); err != nil {
		return fmt.Errorf("failed to reconcile RBAC: %w", err)
	}

	// If no workers are enabled, skip reconciliation
	if len(enabledWorkers) == 0 {
		// No workers enabled, skip reconciliation
		return nil
	}

	// Create ConfigMap with configuration
	configMap := manifests.DataPlaneWorkerAgentConfigMap(hcp.Namespace)
	if _, err := createOrUpdate(ctx, c, configMap, func() error {
		config := generateAgentConfig(enabledWorkers)
		configData, err := json.Marshal(config)
		if err != nil {
			return err
		}

		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		configMap.Data["config.json"] = string(configData)
		return nil
	}); err != nil {
		return fmt.Errorf("failed to reconcile config map: %w", err)
	}

	// Create DaemonSet
	daemonSet := manifests.DataPlaneWorkerAgentDaemonSet(hcp.Namespace)
	if _, err := createOrUpdate(ctx, c, daemonSet, func() error {
		daemonSet.Spec = appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": DataPlaneWorkerAgentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": DataPlaneWorkerAgentName,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: DataPlaneWorkerAgentName,
					DNSPolicy:          corev1.DNSDefault,
					Tolerations:        []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
					Containers: []corev1.Container{
						{
							Name:            DataPlaneWorkerAgentName,
							Image:           cpoImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/bin/control-plane-operator",
							},
							Args: []string{
								"enable-worker-agent",
								"run",
								"--config=/etc/data-plane-worker-agent/config.json",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "agent-config",
									MountPath: "/etc/data-plane-worker-agent",
								},
								{
									Name:      "kubelet-config",
									MountPath: "/var/lib/kubelet",
								},
							},
							TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("50Mi"),
									corev1.ResourceCPU:    resource.MustParse("40m"),
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "agent-config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: DataPlaneWorkerAgentName + "-config",
									},
								},
							},
						},
						{
							Name: "kubelet-config",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet",
									Type: ptr.To(corev1.HostPathDirectory),
								},
							},
						},
					},
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &intstr.IntOrString{Type: intstr.String, StrVal: "10%"},
					MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 0},
				},
			},
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to reconcile daemon set: %w", err)
	}

	return nil
}

func reconcileWorkerAgentRBAC(ctx context.Context, c crclient.Client, createOrUpdate upsert.CreateOrUpdateFN, namespace string, enabledWorkers []string) error {
	log := logr.FromContext(ctx)

	// If no workers are enabled, skip reconciliation
	if len(enabledWorkers) == 0 {
		// remove the RBAC resources
		sa := manifests.DataPlaneWorkerAgentServiceAccount(namespace)
		if err := c.Delete(ctx, sa); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete service account: %w", err)
			}
		}
		clusterRole := manifests.DataPlaneWorkerAgentClusterRole()
		if err := c.Delete(ctx, clusterRole); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete cluster role: %w", err)
			}
		}
		clusterRoleBinding := manifests.DataPlaneWorkerAgentClusterRoleBinding()
		if err := c.Delete(ctx, clusterRoleBinding); err != nil {
			if !apierrors.IsNotFound(err) {
				return fmt.Errorf("failed to delete cluster role binding: %w", err)
			}
		}

		log.Info("No workers enabled, RBAC removed")

		return nil
	}

	// Create ServiceAccount
	sa := manifests.DataPlaneWorkerAgentServiceAccount(namespace)
	if _, err := createOrUpdate(ctx, c, sa, func() error { return nil }); err != nil {
		return fmt.Errorf("failed to reconcile service account: %w", err)
	}

	// Create ClusterRole
	clusterRole := manifests.DataPlaneWorkerAgentClusterRole()
	if _, err := createOrUpdate(ctx, c, clusterRole, func() error {
		clusterRole.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"nodes"},
				Verbs:     []string{"get", "list", "watch"},
			},
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to reconcile cluster role: %w", err)
	}

	// Create ClusterRoleBinding
	clusterRoleBinding := manifests.DataPlaneWorkerAgentClusterRoleBinding()
	if _, err := createOrUpdate(ctx, c, clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      sa.Name,
				Namespace: sa.Namespace,
			},
		}
		return nil
	}); err != nil {
		return fmt.Errorf("failed to reconcile cluster role binding: %w", err)
	}

	return nil
}

func isDataPlaneWorkerAgentEnabled(hcp *hyperv1.HostedControlPlane) bool {
	if hcp == nil || hcp.Annotations == nil {
		return false
	}
	return hcp.Annotations[EnableDataPlaneWorkerAgentAnnotation] == "true"
}

func getEnabledWorkers(hcp *hyperv1.HostedControlPlane) []string {
	if hcp == nil || hcp.Annotations == nil {
		return []string{}
	}

	workersStr := hcp.Annotations[DataPlaneWorkerAgentWorkersAnnotation]
	if workersStr == "" {
		return []string{} // No default workers
	}

	// Split and clean the workers list
	workers := strings.Split(workersStr, ",")
	var cleanedWorkers []string
	for _, worker := range workers {
		worker = strings.TrimSpace(worker)
		if worker != "" {
			cleanedWorkers = append(cleanedWorkers, worker)
		}
	}

	return cleanedWorkers
}

func generateAgentConfig(workers []string) *AgentConfig {
	config := &AgentConfig{
		EnabledWorkers: workers,
		WorkerConfigs:  make(map[string]WorkerConfig),
		LogLevel:       "info",
	}

	// Configure specific workers
	for _, worker := range workers {
		switch worker {
		case "sync-pullsecret":
			config.WorkerConfigs[worker] = WorkerConfig{
				Enabled: true,
				Parameters: map[string]interface{}{
					"kubelet-config-json-path": "/var/lib/kubelet/config.json",
					"global-ps-secret-name":    "global-pull-secret",
					"check-interval":           "30s",
				},
			}
		default:
			// Unknown worker, skip it
			continue
		}
	}

	return config
}
