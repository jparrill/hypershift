package globalps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift/hypershift/support/thirdparty/kubernetes/pkg/credentialprovider"
	"github.com/openshift/hypershift/support/upsert"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-logr/logr"
)

var (
	maxUnavailable = intstr.FromString("10%")
	maxSurge       = intstr.FromInt(0)
)

const (
	NodePullSecretPath = "/var/lib/kubelet/config.json"
)

func ReconcileDaemonSet(ctx context.Context, daemonSet *appsv1.DaemonSet, globalPullSecretBytes []byte, c client.Client, createOrUpdate upsert.CreateOrUpdateFN, cpoImage string) error {
	log, err := logr.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("failed to get logger: %w", err)
	}

	log.Info("Reconciling global pull secret daemon set")

	if _, err := createOrUpdate(ctx, c, daemonSet, func() error {
		daemonSet.Spec = appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": "global-pull-secret-syncer",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": "global-pull-secret-syncer",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:           "global-pull-secret-syncer",
					AutomountServiceAccountToken: ptr.To(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsUser: ptr.To[int64](1000),
					},
					DNSPolicy:   corev1.DNSDefault,
					Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
					Containers: []corev1.Container{
						{
							Name:            "global-pull-secret-syncer",
							Image:           cpoImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/usr/bin/control-plane-operator",
							},
							Args: []string{
								"sync-global-pullsecret",
								"--kubelet-config-json-path=/var/lib/kubelet/config.json",
								"--global-ps-secret-name=global-pull-secret",
								"--check-interval=10s",
							},
							VolumeMounts: []corev1.VolumeMount{
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
							Name: "kubelet-config",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/var/lib/kubelet",
									Type: ptr.To(corev1.HostPathFile),
								},
							},
						},
					},
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDaemonSet{
					MaxUnavailable: &maxUnavailable,
					MaxSurge:       &maxSurge,
				},
			},
		}

		return nil
	}); err != nil {
		return fmt.Errorf("failed to create global pull secret daemon set: %w", err)
	}

	return nil
}

func ValidateAdditionalPullSecret(pullSecret *corev1.Secret) ([]byte, error) {
	var dockerConfigJSON credentialprovider.DockerConfigJSON

	// Validate that the pull secret contains the dockerConfigJson key
	if _, ok := pullSecret.Data[corev1.DockerConfigJsonKey]; !ok {
		return nil, fmt.Errorf("pull secret data is not a valid docker config json")
	}

	// Validate that the pull secret is a valid Docker config JSON
	pullSecretBytes := pullSecret.Data[corev1.DockerConfigJsonKey]
	if err := json.Unmarshal(pullSecretBytes, &dockerConfigJSON); err != nil {
		return nil, fmt.Errorf("invalid docker config json format: %w", err)
	}

	// Validate that the pull secret contains at least one auth entry
	if len(dockerConfigJSON.Auths) == 0 {
		return nil, fmt.Errorf("docker config json must contain at least one auth entry")
	}

	// TODO (jparrill):
	// 	- Validate MachineConfig patches looking for changes over the original pull secret path.
	// 	- Validate the Kubelet flags does not contain a different path for the pull secret.
	// 	- Validate that the pull secret is not conflicting with the original pull secret.

	return pullSecretBytes, nil
}

// MergePullSecrets merges two pull secrets into a single pull secret.
// The additional pull secret is merged with the original pull secret.
// If an auth entry already exists, it will be overwritten.
// The resulting pull secret is returned as a JSON string.
// Not using credentialprovider.DockerConfigJSON because it does not support
// marshalling the auth field.
func MergePullSecrets(originalPullSecret, additionalPullSecret []byte) ([]byte, error) {
	var (
		originalAuths         map[string]any
		additionalAuths       map[string]any
		originalJSON          map[string]any
		additionalJSON        map[string]any
		globalPullSecretBytes []byte
		err                   error
	)

	// Unmarshal original pull secret
	if err = json.Unmarshal(originalPullSecret, &originalJSON); err != nil {
		return nil, fmt.Errorf("invalid original pull secret format: %w", err)
	}
	originalAuths = originalJSON["auths"].(map[string]any)

	// Unmarshal additional pull secret
	if err = json.Unmarshal(additionalPullSecret, &additionalJSON); err != nil {
		return nil, fmt.Errorf("invalid additional pull secret format: %w", err)
	}
	additionalAuths = additionalJSON["auths"].(map[string]any)

	// Merge auths
	for k, v := range additionalAuths {
		originalAuths[k] = v
	}

	// Create final JSON
	finalJSON := map[string]any{
		"auths": originalAuths,
	}

	globalPullSecretBytes, err = json.Marshal(finalJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal merged pull secret: %w", err)
	}

	return globalPullSecretBytes, nil
}
