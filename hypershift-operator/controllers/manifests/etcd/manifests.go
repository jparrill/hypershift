package etcd

import (
	"fmt"

	hypershiftv1beta1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	"github.com/openshift/hypershift/hypershift-operator/controllers/manifests"

	securityv1 "github.com/openshift/api/security/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// we require labeling these cluster-scoped resources so that the controller cleaning them up
// can find them efficiently, as we can't use namespace-based scoping for these objects
const (
	OwningHostedClusterNamespaceLabel = "hypershift.openshift.io/owner.namespace"
	OwningHostedClusterNameLabel      = "hypershift.openshift.io/owner.name"
)

func EtcdSecurityContextConstraints(hc *hypershiftv1beta1.HostedCluster) *securityv1.SecurityContextConstraints {
	controlPlaneNamespace := manifests.HostedControlPlaneNamespace(hc.Namespace, hc.Name)

	return &securityv1.SecurityContextConstraints{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-etcd-fsfreeze", controlPlaneNamespace),
			Labels: map[string]string{
				OwningHostedClusterNamespaceLabel: hc.Namespace,
				OwningHostedClusterNameLabel:      hc.Name,
			},
			Annotations: map[string]string{
				"hypershift.openshift.io/managed":     "true",
				"kubernetes.io/description": "SecurityContextConstraints for HyperShift etcd pods that need CAP_SYS_ADMIN capability for fsfreeze operations during OADP/Velero backup hooks.",
			},
		},
		AllowHostDirVolumePlugin: false,
		AllowHostIPC:             false,
		AllowHostNetwork:         false,
		AllowHostPID:             false,
		AllowHostPorts:           false,
		AllowPrivilegedContainer: false,
		AllowedCapabilities: []corev1.Capability{
			"SYS_ADMIN",
		},
		DefaultAddCapabilities: nil,
		FSGroup: securityv1.FSGroupStrategyOptions{
			Type: securityv1.FSGroupStrategyMustRunAs,
			Ranges: []securityv1.IDRange{
				{
					Min: 1001,
					Max: 1001,
				},
			},
		},
		Priority:               ptr.To[int32](10),
		ReadOnlyRootFilesystem: true,
		RunAsUser: securityv1.RunAsUserStrategyOptions{
			Type:        securityv1.RunAsUserStrategyMustRunAsRange,
			UIDRangeMin: ptr.To[int64](1001),
			UIDRangeMax: ptr.To[int64](1001),
		},
		SELinuxContext: securityv1.SELinuxContextStrategyOptions{
			Type: securityv1.SELinuxStrategyMustRunAs,
		},
		SupplementalGroups: securityv1.SupplementalGroupsStrategyOptions{
			Type: securityv1.SupplementalGroupsStrategyMustRunAs,
			Ranges: []securityv1.IDRange{
				{
					Min: 1001,
					Max: 1001,
				},
			},
		},
		Users: []string{
			fmt.Sprintf("system:serviceaccount:%s:etcd", controlPlaneNamespace),
		},
	}
}