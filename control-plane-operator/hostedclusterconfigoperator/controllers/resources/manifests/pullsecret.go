package manifests

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func PullSecret(ns string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pull-secret",
			Namespace: ns,
		},
	}
}

func PullSecretTargetNamespaces() []string {
	return []string{
		"openshift-config",
		"openshift",
	}
}

func UserProvidedPullSecret(ns string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "additional-pull-secret",
			Namespace: ns,
		},
	}
}

func GlobalPullSecretDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "global-pull-secret-syncer",
			Namespace: "kube-system",
		},
	}
}

func GlobalPullSecret(ns string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "global-pull-secret",
			Namespace: "kube-system",
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}
}
