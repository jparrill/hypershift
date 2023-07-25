package etcd

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	hyperv1 "github.com/openshift/hypershift/api/v1beta1"
	"github.com/openshift/hypershift/support/config"
)

const (
	ipv4EtcdScript = `
/usr/bin/etcd \
--data-dir=/var/lib/data \
--name=${HOSTNAME} \
--initial-advertise-peer-urls=https://${HOSTNAME}.etcd-discovery.${NAMESPACE}.svc:2380 \
--listen-peer-urls=https://${POD_IP}:2380 \
--listen-client-urls=https://${POD_IP}:2379,https://localhost:2379 \
--advertise-client-urls=https://${HOSTNAME}.etcd-client.${NAMESPACE}.svc:2379 \
--listen-metrics-urls=https://0.0.0.0:2382 \
--initial-cluster-token=etcd-cluster \
--initial-cluster=${INITIAL_CLUSTER} \
--initial-cluster-state=new \
--quota-backend-bytes=${QUOTA_BACKEND_BYTES} \
--snapshot-count=10000 \
--peer-client-cert-auth=true \
--peer-cert-file=/etc/etcd/tls/peer/peer.crt \
--peer-key-file=/etc/etcd/tls/peer/peer.key \
--peer-trusted-ca-file=/etc/etcd/tls/etcd-ca/ca.crt \
--client-cert-auth=true \
--cert-file=/etc/etcd/tls/server/server.crt \
--key-file=/etc/etcd/tls/server/server.key \
--trusted-ca-file=/etc/etcd/tls/etcd-ca/ca.crt
`

	ipv6EtcdScript = `
/usr/bin/etcd \
--data-dir=/var/lib/data \
--name=${HOSTNAME} \
--initial-advertise-peer-urls=https://${HOSTNAME}.etcd-discovery.${NAMESPACE}.svc:2380 \
--listen-peer-urls=https://[${POD_IP}]:2380 \
--listen-client-urls=https://[${POD_IP}]:2379,https://localhost:2379 \
--advertise-client-urls=https://${HOSTNAME}.etcd-client.${NAMESPACE}.svc:2379 \
--listen-metrics-urls=https://[::]:2382 \
--initial-cluster-token=etcd-cluster \
--initial-cluster=${INITIAL_CLUSTER} \
--initial-cluster-state=new \
--quota-backend-bytes=${QUOTA_BACKEND_BYTES} \
--snapshot-count=10000 \
--peer-client-cert-auth=true \
--peer-cert-file=/etc/etcd/tls/peer/peer.crt \
--peer-key-file=/etc/etcd/tls/peer/peer.key \
--peer-trusted-ca-file=/etc/etcd/tls/etcd-ca/ca.crt \
--client-cert-auth=true \
--cert-file=/etc/etcd/tls/server/server.crt \
--key-file=/etc/etcd/tls/server/server.key \
--trusted-ca-file=/etc/etcd/tls/etcd-ca/ca.crt
`
)

func TestBuildEtcdInitContainer(t *testing.T) {
	tests := []struct {
		name        string
		params      EtcdParams
		expectedEnv []corev1.EnvVar
	}{
		{
			name: "single replica container env as expected",
			params: EtcdParams{
				EtcdImage: "animage",
				DeploymentConfig: config.DeploymentConfig{
					Replicas: 1,
				},
				StorageSpec: hyperv1.ManagedEtcdStorageSpec{
					RestoreSnapshotURL: []string{"arestoreurl"},
				},
			},
			expectedEnv: []corev1.EnvVar{
				{
					Name:  "RESTORE_URL_ETCD_0",
					Value: "arestoreurl",
				},
			},
		},
		{
			name: "three replica container env as expected",
			params: EtcdParams{
				EtcdImage: "animage",
				DeploymentConfig: config.DeploymentConfig{
					Replicas: 3,
				},
				StorageSpec: hyperv1.ManagedEtcdStorageSpec{
					RestoreSnapshotURL: []string{"u1", "u2", "u3"},
				},
			},
			expectedEnv: []corev1.EnvVar{
				{
					Name:  "RESTORE_URL_ETCD_0",
					Value: "u1",
				},
				{
					Name:  "RESTORE_URL_ETCD_1",
					Value: "u2",
				},
				{
					Name:  "RESTORE_URL_ETCD_2",
					Value: "u3",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			c := corev1.Container{}
			buildEtcdInitContainer(&tt.params)(&c)
			g.Expect(c.Env).Should(ConsistOf(tt.expectedEnv))
		})
	}
}

func TestBuildEtcdContainer(t *testing.T) {
	tests := []struct {
		name            string
		namespace       string
		params          EtcdParams
		expectedCommand []string
	}{
		{
			name:      "given ipv4 environment, it should return a single replica container with ipv4 script",
			namespace: "test-ns",
			params: EtcdParams{
				EtcdImage: "animage",
				DeploymentConfig: config.DeploymentConfig{
					Replicas: 1,
				},
				StorageSpec: hyperv1.ManagedEtcdStorageSpec{
					RestoreSnapshotURL: []string{"arestoreurl"},
				},
				IPv6: false,
			},
			expectedCommand: []string{"/bin/sh", "-c", ipv4EtcdScript},
		},
		{
			name:      "given ipv6 environment, it should return a three replica container with ipv6 script",
			namespace: "test-ns",
			params: EtcdParams{
				EtcdImage: "animage",
				DeploymentConfig: config.DeploymentConfig{
					Replicas: 1,
				},
				StorageSpec: hyperv1.ManagedEtcdStorageSpec{
					RestoreSnapshotURL: []string{"u1", "u2", "u3"},
				},
				IPv6: true,
			},
			expectedCommand: []string{"/bin/sh", "-c", ipv6EtcdScript},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewGomegaWithT(t)
			c := corev1.Container{}
			buildEtcdContainer(&tt.params, tt.namespace)(&c)
			g.Expect(c.Command).Should(ConsistOf(tt.expectedCommand))
		})
	}
}
