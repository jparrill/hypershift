package manifests

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func DataPlaneWorkerAgentDaemonSet(namespace string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-plane-worker-agent",
			Namespace: namespace,
		},
	}
}

func DataPlaneWorkerAgentServiceAccount(namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-plane-worker-agent",
			Namespace: namespace,
		},
	}
}

func DataPlaneWorkerAgentClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "data-plane-worker-agent",
		},
	}
}

func DataPlaneWorkerAgentClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "data-plane-worker-agent",
		},
	}
}

func DataPlaneWorkerAgentConfigMap(namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "data-plane-worker-agent-config",
			Namespace: namespace,
		},
	}
}
