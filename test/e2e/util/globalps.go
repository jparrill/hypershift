package util

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	CRIOConfigVerifierDaemonSetName = "crio-config-verifier"
	CRIOConfigVerifierNamespace     = "kube-system"
	CRIOGlobalAuthFilePath          = "/etc/hypershift/global-pull-secret.json"
	CRIODropInConfigPath            = "/etc/crio/crio.conf.d/99-global-pull-secret.conf"
)

// CreateCRIOConfigVerifierDaemonSet creates a DaemonSet that verifies the CRIO global authentication configuration
// on all nodes of the cluster, checking that CRIO is properly configured to use the global pull secret
func CreateCRIOConfigVerifierDaemonSet(ctx context.Context, guestClient crclient.Client, dsImage string) error {

	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRIOConfigVerifierDaemonSetName,
			Namespace: CRIOConfigVerifierNamespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": CRIOConfigVerifierDaemonSetName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"name": CRIOConfigVerifierDaemonSetName,
					},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{},
					DNSPolicy:       corev1.DNSDefault,
					Tolerations:     []corev1.Toleration{{Operator: corev1.TolerationOpExists}},
					Containers: []corev1.Container{
						{
							Name:            CRIOConfigVerifierDaemonSetName,
							Image:           dsImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command: []string{
								"/bin/sh", "-c",
							},
							Args: []string{
								fmt.Sprintf(`
									echo "Starting CRIO global pull secret verification..."
									echo "Checking CRIO drop-in config: %s"
									echo "Checking global auth file: %s"

									# Check if global pull secret is configured via CRI-O
									if [ -f %s ]; then
										echo "SUCCESS: CRIO drop-in config exists at %s"
										echo "--- CRIO drop-in config content ---"
										cat %s
										echo "--- End of CRIO config ---"

										# Verify the config points to the expected global auth file
										if ! grep -q 'global_auth_file.*%s' %s; then
											echo "ERROR: CRIO config does not point to expected global auth file"
											echo "Expected: global_auth_file pointing to %s"
											echo "Current config content:"
											cat %s
											exit 1
										fi
										echo "SUCCESS: CRIO config correctly points to global auth file"

										# Verify the global auth file exists and is valid
										if [ ! -f %s ]; then
											echo "ERROR: Global auth file does not exist at %s"
											echo "Expected path: %s"
											echo "Directory listing of /etc/hypershift:"
											ls -la /etc/hypershift/ || echo "Cannot list /etc/hypershift directory"
											exit 1
										fi

										echo "SUCCESS: Global auth file exists at %s"
										echo "File size: $(stat -c%%s %s 2>/dev/null || stat -f%%z %s 2>/dev/null) bytes"

										# Verify that the file contains valid JSON structure
										if ! grep -q '"auths"' %s; then
											echo "ERROR: Global auth file does not contain 'auths' field"
											echo "File content (first 500 chars):"
											head -c 500 %s || echo "Cannot read file"
											exit 1
										fi

										echo "SUCCESS: Global auth file contains valid 'auths' field"
										echo "SUCCESS: CRIO global pull secret is properly configured and functional"
									else
										echo "INFO: CRIO drop-in config does not exist at %s"
										echo "This indicates global pull secret is not configured"
										echo "Checking if directory exists:"
										ls -la /etc/crio/crio.conf.d/ || echo "Directory /etc/crio/crio.conf.d/ does not exist"
										echo "This is valid when no global pull secret is configured"
										echo "SUCCESS: Using default container runtime authentication"
									fi

									echo "SUCCESS: CRIO pull secret verification completed"

									# Keep the pod running with infinite loop
									echo "Starting infinite loop to keep pod running..."
									while true; do
										echo "Pod is still running... $(date)"
										sleep 30
									done
								`, CRIODropInConfigPath, CRIOGlobalAuthFilePath, CRIODropInConfigPath, CRIODropInConfigPath, CRIODropInConfigPath, CRIOGlobalAuthFilePath, CRIODropInConfigPath, CRIOGlobalAuthFilePath, CRIODropInConfigPath, CRIOGlobalAuthFilePath, CRIOGlobalAuthFilePath, CRIOGlobalAuthFilePath, CRIOGlobalAuthFilePath, CRIOGlobalAuthFilePath, CRIOGlobalAuthFilePath, CRIOGlobalAuthFilePath, CRIOGlobalAuthFilePath, CRIODropInConfigPath),
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: ptr.To(true),
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "crio-config",
									MountPath: "/etc/crio",
								},
								{
									Name:      "hypershift-config",
									MountPath: "/etc/hypershift",
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
							Name: "crio-config",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/crio",
									Type: ptr.To(corev1.HostPathDirectoryOrCreate),
								},
							},
						},
						{
							Name: "hypershift-config",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/etc/hypershift",
									Type: ptr.To(corev1.HostPathDirectoryOrCreate),
								},
							},
						},
					},
				},
			},
		},
	}

	return guestClient.Create(ctx, daemonSet)
}

// WaitForCRIOConfigVerifierDaemonSet waits for the DaemonSet to be ready
func WaitForCRIOConfigVerifierDaemonSet(ctx context.Context, guestClient crclient.Client) error {
	return wait.PollUntilContextTimeout(ctx, 10*time.Second, 20*time.Minute, true,
		func(ctx context.Context) (done bool, err error) {
			ds := &appsv1.DaemonSet{}
			if err := guestClient.Get(ctx, crclient.ObjectKey{Name: CRIOConfigVerifierDaemonSetName, Namespace: CRIOConfigVerifierNamespace}, ds); err != nil {
				return false, err
			}
			return ds.Status.NumberReady == ds.Status.DesiredNumberScheduled, nil
		})
}

// VerifyCRIOConfigWithDaemonSet implements complete verification of CRIO global authentication configuration using DaemonSet
func VerifyCRIOConfigWithDaemonSet(t *testing.T, ctx context.Context, guestClient crclient.Client, dsImage string) {
	g := NewWithT(t)

	// Create the DaemonSet
	t.Log("Creating CRIO config verifier DaemonSet")
	err := CreateCRIOConfigVerifierDaemonSet(ctx, guestClient, dsImage)
	g.Expect(err).NotTo(HaveOccurred(), "failed to create CRIO config verifier DaemonSet")

	// Wait for the DaemonSet to be ready
	t.Log("Waiting for DaemonSet to be ready")
	err = WaitForCRIOConfigVerifierDaemonSet(ctx, guestClient)
	g.Expect(err).NotTo(HaveOccurred(), "failed to wait for CRIO config verifier DaemonSet")

	// Verify that all DaemonSet pods are running
	t.Log("Verifying all DaemonSet pods are running")
	EventuallyObjects(t, ctx, "DaemonSet pods to be running", func(ctx context.Context) ([]*corev1.Pod, error) {
		pods := &corev1.PodList{}
		err := guestClient.List(ctx, pods, &crclient.ListOptions{
			Namespace: CRIOConfigVerifierNamespace,
			LabelSelector: labels.Set(map[string]string{
				"name": CRIOConfigVerifierDaemonSetName,
			}).AsSelector(),
		})
		if err != nil {
			return nil, err
		}
		var items []*corev1.Pod
		for i := range pods.Items {
			items = append(items, &pods.Items[i])
		}
		return items, nil
	}, nil, []Predicate[*corev1.Pod]{func(pod *corev1.Pod) (done bool, reasons string, err error) {
		return pod.Status.Phase == corev1.PodRunning, fmt.Sprintf("Pod has phase %s", pod.Status.Phase), nil
	}}, WithInterval(5*time.Second), WithTimeout(30*time.Minute))

	// Clean up the DaemonSet after verification
	t.Log("Cleaning up CRIO config verifier DaemonSet")
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CRIOConfigVerifierDaemonSetName,
			Namespace: CRIOConfigVerifierNamespace,
		},
	}
	g.Expect(guestClient.Delete(ctx, daemonSet)).To(Succeed())

}
