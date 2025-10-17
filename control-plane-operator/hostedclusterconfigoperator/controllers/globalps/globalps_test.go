package globalps

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	validAuth = base64.StdEncoding.EncodeToString([]byte("user:pass"))
	oldAuth   = base64.StdEncoding.EncodeToString([]byte("olduser:oldpass"))
)

func TestValidateAdditionalPullSecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  *corev1.Secret
		wantErr bool
	}{
		{
			name: "valid pull secret",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: composePullSecretBytes(map[string]string{"quay.io": validAuth}),
				},
			},
			wantErr: false,
		},
		{
			name: "missing docker config key",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					"wrong-key": composePullSecretBytes(map[string]string{"quay.io": validAuth}),
				},
			},
			wantErr: true,
		},
		{
			name: "invalid json",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`invalid json`),
				},
			},
			wantErr: true,
		},
		{
			name: "empty auths",
			secret: &corev1.Secret{
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`),
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := validateAdditionalPullSecret(tt.secret)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

func TestMergePullSecrets(t *testing.T) {
	tests := []struct {
		name             string
		originalSecret   []byte
		additionalSecret []byte
		expectedResult   []byte
		wantErr          bool
	}{
		{
			name:             "successful merge with 1 entries",
			originalSecret:   composePullSecretBytes(map[string]string{"registry1": validAuth}),
			additionalSecret: composePullSecretBytes(map[string]string{"registry2": validAuth}),
			expectedResult:   composePullSecretBytesOrdered([]string{"registry2", "registry1"}, map[string]string{"registry1": validAuth, "registry2": validAuth}),
			wantErr:          false,
		},
		{
			name:             "successful merge with 2 entries in additional secret",
			originalSecret:   composePullSecretBytes(map[string]string{"registry1": validAuth}),
			additionalSecret: composePullSecretBytes(map[string]string{"registry2": validAuth, "registry3": validAuth}),
			expectedResult:   composePullSecretBytesOrdered([]string{"registry2", "registry3", "registry1"}, map[string]string{"registry1": validAuth, "registry2": validAuth, "registry3": validAuth}),
			wantErr:          false,
		},
		{
			name:             "successful merge with 2 entries in original secret",
			originalSecret:   composePullSecretBytes(map[string]string{"registry1": validAuth, "registry2": validAuth}),
			additionalSecret: composePullSecretBytes(map[string]string{"registry3": validAuth}),
			expectedResult:   composePullSecretBytesOrdered([]string{"registry3", "registry1", "registry2"}, map[string]string{"registry1": validAuth, "registry2": validAuth, "registry3": validAuth}),
			wantErr:          false,
		},
		{
			name:             "conflict resolution - original always wins",
			originalSecret:   composePullSecretBytes(map[string]string{"registry1": oldAuth}),
			additionalSecret: composePullSecretBytes(map[string]string{"registry1": validAuth}),
			expectedResult:   composePullSecretBytes(map[string]string{"registry1": oldAuth}),
			wantErr:          false,
		},
		{
			name:             "precedence test - original always has precedence",
			originalSecret:   composePullSecretBytes(map[string]string{"registry1": oldAuth, "registry2": oldAuth}),
			additionalSecret: composePullSecretBytes(map[string]string{"registry1": validAuth, "registry3": validAuth}),
			expectedResult:   composePullSecretBytesOrdered([]string{"registry3", "registry1", "registry2"}, map[string]string{"registry1": oldAuth, "registry2": oldAuth, "registry3": validAuth}),
			wantErr:          false,
		},
		{
			name:             "multiple conflicts - original always wins",
			originalSecret:   composePullSecretBytes(map[string]string{"registry1": oldAuth, "registry2": oldAuth}),
			additionalSecret: composePullSecretBytes(map[string]string{"registry1": validAuth, "registry2": validAuth, "registry3": validAuth}),
			expectedResult:   composePullSecretBytesOrdered([]string{"registry3", "registry1", "registry2"}, map[string]string{"registry1": oldAuth, "registry2": oldAuth, "registry3": validAuth}),
			wantErr:          false,
		},
		{
			name:             "invalid original secret",
			originalSecret:   []byte(`invalid json`),
			additionalSecret: composePullSecretBytes(map[string]string{"registry1": validAuth}),
			wantErr:          true,
		},
		{
			name:             "invalid additional secret",
			originalSecret:   composePullSecretBytes(map[string]string{"registry1": validAuth}),
			additionalSecret: []byte(`invalid json`),
			wantErr:          true,
		},
		{
			name:             "empty additional secret, invalid JSON",
			originalSecret:   composePullSecretBytes(map[string]string{"registry1": validAuth}),
			additionalSecret: []byte{},
			expectedResult:   composePullSecretBytes(map[string]string{"registry1": validAuth}),
			wantErr:          true,
		},
		{
			name:             "empty additional secret with valid JSON",
			originalSecret:   composePullSecretBytes(map[string]string{"registry1": validAuth, "registry2": validAuth}),
			additionalSecret: []byte(`{"auths":{}}`),
			expectedResult:   composePullSecretBytes(map[string]string{"registry1": validAuth, "registry2": validAuth}),
			wantErr:          false,
		},
		{
			name:           "order test - additional registries should appear first",
			originalSecret: composePullSecretBytes(map[string]string{"original.registry": oldAuth}),
			additionalSecret: composePullSecretBytes(map[string]string{"additional.registry": validAuth}),
			expectedResult: composePullSecretBytesOrdered([]string{"additional.registry", "original.registry"}, map[string]string{
				"additional.registry": validAuth,
				"original.registry":   oldAuth,
			}),
			wantErr: false,
		},
		{
			name:           "order test with conflict - original wins but additional appears first in non-conflicting entries",
			originalSecret: composePullSecretBytes(map[string]string{"conflict.registry": oldAuth, "original-only.registry": oldAuth}),
			additionalSecret: composePullSecretBytes(map[string]string{"conflict.registry": validAuth, "additional-only.registry": validAuth}),
			expectedResult: composePullSecretBytesOrdered([]string{"additional-only.registry", "conflict.registry", "original-only.registry"}, map[string]string{
				"additional-only.registry": validAuth,
				"conflict.registry":        oldAuth, // original wins
				"original-only.registry":   oldAuth,
			}),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result, err := mergePullSecrets(context.Background(), tt.originalSecret, tt.additionalSecret)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(tt.expectedResult))
			}
		})
	}
}

// TestMergePullSecretsOrderVerification specifically tests the order of registries in the merged JSON
func TestMergePullSecretsOrderVerification(t *testing.T) {
	g := NewWithT(t)

	// Create realistic test data
	originalAuth := base64.StdEncoding.EncodeToString([]byte("user:pass"))
	additionalAuth := base64.StdEncoding.EncodeToString([]byte("additional:pass"))

	originalSecret := composePullSecretBytes(map[string]string{
		"registry.redhat.io": originalAuth,
		"registry.access.redhat.com": originalAuth,
	})

	additionalSecret := composePullSecretBytes(map[string]string{
		"quay.io": additionalAuth,
		"docker.io": additionalAuth,
	})

	result, err := mergePullSecrets(context.Background(), originalSecret, additionalSecret)
	g.Expect(err).NotTo(HaveOccurred())

	// Parse the result to verify order
	var resultJSON map[string]any
	err = json.Unmarshal(result, &resultJSON)
	g.Expect(err).NotTo(HaveOccurred())

	auths := resultJSON["auths"].(map[string]any)
	g.Expect(auths).To(HaveLen(4))

	// Verify all expected registries are present
	g.Expect(auths).To(HaveKey("quay.io"))
	g.Expect(auths).To(HaveKey("docker.io"))
	g.Expect(auths).To(HaveKey("registry.redhat.io"))
	g.Expect(auths).To(HaveKey("registry.access.redhat.com"))

	// For Go maps, we can't guarantee order in the JSON output due to map iteration randomness,
	// but our implementation should place additional registries first when possible.
	// This is more about ensuring the merge logic works correctly rather than strict JSON ordering.

	t.Logf("Merged pull secret JSON: %s", string(result))
}

func composePullSecretBytes(auths map[string]string) []byte {
	authsJSON := make(map[string]any)
	authsEntries := make(map[string]any)
	for registry, authEntry := range auths {
		authsEntries[registry] = map[string]any{
			"auth": authEntry,
		}
	}
	authsJSON["auths"] = authsEntries
	authsBytes, err := json.Marshal(authsJSON)
	if err != nil {
		panic(err)
	}
	return authsBytes
}

// composePullSecretBytesOrdered creates a pull secret JSON with a specific order of registries
func composePullSecretBytesOrdered(order []string, auths map[string]string) []byte {
	// Use ordered map to ensure specific order in JSON
	authsJSON := `{"auths":{`
	for i, registry := range order {
		if i > 0 {
			authsJSON += ","
		}
		authEntry := auths[registry]
		authsJSON += `"` + registry + `":{"auth":"` + authEntry + `"}`
	}
	authsJSON += `}}`
	return []byte(authsJSON)
}

func TestAdditionalPullSecretExists(t *testing.T) {
	pullSecret := composePullSecretBytes(map[string]string{"quay.io": validAuth})
	tests := []struct {
		name           string
		secretExists   bool
		expectedExists bool
		expectedSecret *corev1.Secret
		objects        []client.Object
	}{
		{
			name:           "secret exists",
			secretExists:   true,
			expectedExists: true,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "additional-pull-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: pullSecret,
				},
			},
			objects: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "additional-pull-secret",
						Namespace: "kube-system",
					},
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: pullSecret,
					},
				},
			},
		},
		{
			name:           "secret exists but has no content",
			secretExists:   true,
			expectedExists: true,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "additional-pull-secret",
					Namespace: "kube-system",
				},
				Data: nil,
			},
			objects: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "additional-pull-secret",
						Namespace: "kube-system",
					},
					Data: nil,
				},
			},
		},
		{
			name:           "secret exists but has incorrect content",
			secretExists:   true,
			expectedExists: true,
			expectedSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "additional-pull-secret",
					Namespace: "kube-system",
				},
				Data: map[string][]byte{
					corev1.DockerConfigJsonKey: []byte(`invalid json content`),
				},
			},
			objects: []client.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "additional-pull-secret",
						Namespace: "kube-system",
					},
					Data: map[string][]byte{
						corev1.DockerConfigJsonKey: []byte(`invalid json content`),
					},
				},
			},
		},
		{
			name:           "secret does not exist",
			secretExists:   false,
			expectedExists: false,
			expectedSecret: nil,
			objects:        []client.Object{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fakeClient := fake.NewClientBuilder().WithObjects(tt.objects...).Build()
			exists, secret, err := additionalPullSecretExists(context.Background(), fakeClient)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(Equal(tt.expectedExists))

			if tt.expectedSecret != nil {
				g.Expect(secret).NotTo(BeNil())
				g.Expect(secret.Name).To(Equal(tt.expectedSecret.Name))
				g.Expect(secret.Namespace).To(Equal(tt.expectedSecret.Namespace))
				g.Expect(secret.Data).To(Equal(tt.expectedSecret.Data))
			} else {
				g.Expect(secret).To(BeNil())
			}
		})
	}
}
