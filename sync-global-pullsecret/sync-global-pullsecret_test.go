package syncglobalpullsecret

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/go-logr/logr"
	"go.uber.org/mock/gomock"
)

func TestDecidePullSecretStrategy(t *testing.T) {
	tests := []struct {
		name                      string
		globalPullSecretExists    bool
		globalPullSecretContent   string
		originalPullSecretContent string
		expectedUseGlobal         bool
		expectedError             bool
		description               string
	}{
		{
			name:                      "global pull secret does not exist",
			description:               "when global pull secret file doesn't exist, should use original",
			globalPullSecretExists:    false,
			globalPullSecretContent:   "",
			originalPullSecretContent: generatePullSecretWithAuth("registry.redhat.io", "test:test"),
			expectedUseGlobal:         false,
			expectedError:             false,
		},
		{
			name:                      "global pull secret exists and has content",
			description:               "when global pull secret exists with content, should use global",
			globalPullSecretExists:    true,
			globalPullSecretContent:   generatePullSecretWithMultipleAuths(map[string]string{"registry.redhat.io": "test:test", "quay.io": "another:auth"}),
			originalPullSecretContent: generatePullSecretWithAuth("registry.redhat.io", "test:test"),
			expectedUseGlobal:         true,
			expectedError:             false,
		},
		{
			name:                      "global pull secret exists but is empty",
			description:               "when global pull secret exists but is empty, should use original",
			globalPullSecretExists:    true,
			globalPullSecretContent:   "",
			originalPullSecretContent: generatePullSecretWithAuth("registry.redhat.io", "test:test"),
			expectedUseGlobal:         false,
			expectedError:             false,
		},
		{
			name:                      "global pull secret exists but is invalid",
			description:               "when global pull secret exists but is invalid JSON, should use original",
			globalPullSecretExists:    true,
			globalPullSecretContent:   `invalid json`,
			originalPullSecretContent: generatePullSecretWithAuth("registry.redhat.io", "test:test"),
			expectedUseGlobal:         false,
			expectedError:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create a temporary directory for test files
			tempDir, err := os.MkdirTemp("", "sync-pullsecret-test-*")
			g.Expect(err).To(BeNil())
			defer os.RemoveAll(tempDir)

			// Create temporary directories
			originalSecretDir := filepath.Join(tempDir, "original-pull-secret")
			globalSecretDir := filepath.Join(tempDir, "global-pull-secret")

			err = os.MkdirAll(originalSecretDir, 0755)
			g.Expect(err).To(BeNil())
			err = os.MkdirAll(globalSecretDir, 0755)
			g.Expect(err).To(BeNil())

			// Create original pull secret file
			originalSecretFile := filepath.Join(originalSecretDir, ".dockerconfigjson")
			err = os.WriteFile(originalSecretFile, []byte(tt.originalPullSecretContent), 0600)
			g.Expect(err).To(BeNil())

			// Create global pull secret file if it should exist
			globalSecretFile := filepath.Join(globalSecretDir, ".dockerconfigjson")
			if tt.globalPullSecretExists {
				err = os.WriteFile(globalSecretFile, []byte(tt.globalPullSecretContent), 0600)
				g.Expect(err).To(BeNil())
			}

			// Create syncer for testing
			syncer := &GlobalPullSecretSyncer{
				hcpOriginalPullSecretFilePath: originalSecretFile,
				hcpGlobalPullSecretFilePath:   globalSecretFile,
				log:                           logr.Discard(),
			}

			// Run decidePullSecretStrategy
			useGlobal, finalPullSecret, err := syncer.decidePullSecretStrategy()

			// Check error expectations
			if tt.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).To(BeNil())
				g.Expect(useGlobal).To(Equal(tt.expectedUseGlobal))

				// Verify the final pull secret content
				if tt.expectedUseGlobal {
					g.Expect(string(finalPullSecret)).To(Equal(tt.globalPullSecretContent))
				} else {
					g.Expect(string(finalPullSecret)).To(Equal(tt.originalPullSecretContent))
				}
			}
		})
	}
}

func TestConfigureCRIO(t *testing.T) {
	tests := []struct {
		name                     string
		useGlobalPullSecret      bool
		pullSecretContent        string
		expectedCRIOConfigExists bool
		setupExistingFiles       func(string, string) // crioConfigPath, globalSecretPath
		expectedCRIORestart      bool
		description              string
	}{
		{
			name:                     "use global pull secret - first time",
			description:              "when using global pull secret for first time, should create CRIO config and restart",
			useGlobalPullSecret:      true,
			pullSecretContent:        generatePullSecretWithAuth("registry.redhat.io", "test:test"),
			expectedCRIOConfigExists: true,
			expectedCRIORestart:      true,
			setupExistingFiles:       func(string, string) {}, // no existing files
		},
		{
			name:                     "use global pull secret - same content",
			description:              "when using global pull secret with same content, should not restart",
			useGlobalPullSecret:      true,
			pullSecretContent:        generatePullSecretWithAuth("registry.redhat.io", "test:test"),
			expectedCRIOConfigExists: true,
			expectedCRIORestart:      false,
			setupExistingFiles: func(crioPath, secretPath string) {
				// Create existing files with same content
				os.WriteFile(secretPath, []byte(generatePullSecretWithAuth("registry.redhat.io", "test:test")), 0600)
				os.WriteFile(crioPath, []byte(`[crio.image]
global_auth_file = "`+secretPath+`"
`), 0644)
			},
		},
		{
			name:                     "use original pull secret - remove existing config",
			description:              "when switching to original pull secret, should remove CRIO config and restart",
			useGlobalPullSecret:      false,
			pullSecretContent:        generatePullSecretWithAuth("registry.redhat.io", "test:test"),
			expectedCRIOConfigExists: false,
			expectedCRIORestart:      true,
			setupExistingFiles: func(crioPath, secretPath string) {
				// Create existing files to be removed
				os.WriteFile(secretPath, []byte(generatePullSecretWithAuth("old", "test:old")), 0600)
				os.WriteFile(crioPath, []byte(`[crio.image]
global_auth_file = "`+secretPath+`"
`), 0644)
			},
		},
		{
			name:                     "use original pull secret - no existing config",
			description:              "when using original pull secret and no config exists, should not restart",
			useGlobalPullSecret:      false,
			pullSecretContent:        generatePullSecretWithAuth("registry.redhat.io", "test:test"),
			expectedCRIOConfigExists: false,
			expectedCRIORestart:      false,
			setupExistingFiles:       func(string, string) {}, // no existing files
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create a temporary directory for test files
			tempDir, err := os.MkdirTemp("", "sync-pullsecret-test-*")
			g.Expect(err).To(BeNil())
			defer os.RemoveAll(tempDir)

			// Create temporary directories for CRIO and hypershift config
			crioDir := filepath.Join(tempDir, "crio", "crio.conf.d")
			hypershiftDir := filepath.Join(tempDir, "hypershift")

			err = os.MkdirAll(crioDir, 0755)
			g.Expect(err).To(BeNil())
			err = os.MkdirAll(hypershiftDir, 0755)
			g.Expect(err).To(BeNil())

			crioConfigPath := filepath.Join(crioDir, "99-global-pull-secret.conf")
			globalSecretPath := filepath.Join(hypershiftDir, "global-pull-secret.json")

			// Setup existing files if needed
			tt.setupExistingFiles(crioConfigPath, globalSecretPath)

			// Track if CRIO restart was called
			crioRestartCalled := false

			// Create syncer for testing with test-specific paths
			syncer := &GlobalPullSecretSyncer{
				crioGlobalPullSecretConfigPath: crioConfigPath,
				globalPullSecretPath:           globalSecretPath,
				log:                            logr.Discard(),
				signalDBUSToRestartCrioFunc: func() error {
					crioRestartCalled = true
					return nil
				},
			}

			// Run configureCRIO
			err = syncer.configureCRIO(tt.useGlobalPullSecret, []byte(tt.pullSecretContent))
			g.Expect(err).To(BeNil())

			// Verify if CRIO restart was called as expected
			g.Expect(crioRestartCalled).To(Equal(tt.expectedCRIORestart),
				"CRIO restart expectation mismatch: expected %v, got %v", tt.expectedCRIORestart, crioRestartCalled)

			// Check if CRIO config file exists as expected
			_, err = os.Stat(syncer.crioGlobalPullSecretConfigPath)
			if tt.expectedCRIOConfigExists {
				g.Expect(err).To(BeNil(), "CRIO config file should exist")

				// Verify the content of CRIO config file
				content, err := os.ReadFile(syncer.crioGlobalPullSecretConfigPath)
				g.Expect(err).To(BeNil())
				expectedConfig := fmt.Sprintf(`[crio.image]
global_auth_file = "%s"
`, syncer.globalPullSecretPath)
				g.Expect(string(content)).To(Equal(expectedConfig))

				// Verify global pull secret file exists
				_, err = os.Stat(syncer.globalPullSecretPath)
				g.Expect(err).To(BeNil(), "Global pull secret file should exist")

				// Verify global pull secret content
				secretContent, err := os.ReadFile(syncer.globalPullSecretPath)
				g.Expect(err).To(BeNil())
				g.Expect(string(secretContent)).To(Equal(tt.pullSecretContent))
			} else {
				g.Expect(os.IsNotExist(err)).To(BeTrue(), "CRIO config file should not exist")

				// Verify global pull secret file is also removed
				_, err = os.Stat(syncer.globalPullSecretPath)
				g.Expect(os.IsNotExist(err)).To(BeTrue(), "Global pull secret file should not exist")
			}
		})
	}
}

func TestRestartSystemdUnit(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockdbusConn)
		expectedError string
		description   string
	}{
		{
			name: "Success",
			setupMock: func(mock *MockdbusConn) {
				mock.EXPECT().
					RestartUnit(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(name, mode string, ch chan<- string) (int, error) {
						go func() { ch <- "done" }()
						return 0, nil
					})
			},
			expectedError: "",
			description:   "systemd job completed successfully",
		},
		{
			name: "RestartUnit returns an error",
			setupMock: func(mock *MockdbusConn) {
				mock.EXPECT().
					RestartUnit(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(0, fmt.Errorf("dbus error"))
			},
			expectedError: "failed to restart kubelet.service: dbus error",
			description:   "dbus call itself failed",
		},
		{
			name: "Job failed",
			setupMock: func(mock *MockdbusConn) {
				mock.EXPECT().
					RestartUnit(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(name, mode string, ch chan<- string) (int, error) {
						go func() { ch <- "failed" }()
						return 0, nil
					})
			},
			expectedError: "failed to restart kubelet.service, result: failed",
			description:   "systemd job failed",
		},
		{
			name: "Job timeout",
			setupMock: func(mock *MockdbusConn) {
				mock.EXPECT().
					RestartUnit(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(name, mode string, ch chan<- string) (int, error) {
						go func() { ch <- "timeout" }()
						return 0, nil
					})
			},
			expectedError: "failed to restart kubelet.service, result: timeout",
			description:   "systemd job timed out",
		},
		{
			name: "Job canceled",
			setupMock: func(mock *MockdbusConn) {
				mock.EXPECT().
					RestartUnit(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(name, mode string, ch chan<- string) (int, error) {
						go func() { ch <- "canceled" }()
						return 0, nil
					})
			},
			expectedError: "failed to restart kubelet.service, result: canceled",
			description:   "systemd job was canceled",
		},
		{
			name: "Job dependency failed",
			setupMock: func(mock *MockdbusConn) {
				mock.EXPECT().
					RestartUnit(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(name, mode string, ch chan<- string) (int, error) {
						go func() { ch <- "dependency" }()
						return 0, nil
					})
			},
			expectedError: "failed to restart kubelet.service, result: dependency",
			description:   "systemd job dependency failed",
		},
		{
			name: "Job skipped",
			setupMock: func(mock *MockdbusConn) {
				mock.EXPECT().
					RestartUnit(gomock.Any(), gomock.Any(), gomock.Any()).
					DoAndReturn(func(name, mode string, ch chan<- string) (int, error) {
						go func() { ch <- "skipped" }()
						return 0, nil
					})
			},
			expectedError: "failed to restart kubelet.service, result: skipped",
			description:   "systemd job was skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mock := NewMockdbusConn(ctrl)
			tt.setupMock(mock)

			err := restartSystemdUnit(mock, kubeletServiceUnit)
			if err != nil {
				if tt.expectedError == "" {
					t.Errorf("unexpected error: %v", err)
				} else if err.Error() != tt.expectedError {
					t.Errorf("expected error '%s', got '%s'", tt.expectedError, err.Error())
				}
			} else if tt.expectedError != "" {
				t.Errorf("expected error '%s', got nil", tt.expectedError)
			}
		})
	}
}

func TestValidateDockerConfigJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectError bool
		description string
	}{
		{
			name:        "valid docker config with single auth",
			input:       []byte(generatePullSecretWithAuth("test.registry.com", "test:test")),
			expectError: false,
			description: "valid JSON with auths key containing single registry",
		},
		{
			name:        "valid docker config with multiple auths",
			input:       []byte(generatePullSecretWithMultipleAuths(map[string]string{"registry1.com": "test:test", "registry2.com": "another:auth"})),
			expectError: false,
			description: "valid JSON with auths key containing multiple registries",
		},
		{
			name:        "valid docker config with empty auths",
			input:       []byte(`{"auths":{}}`),
			expectError: false,
			description: "valid JSON with empty auths object",
		},
		{
			name:        "valid docker config with additional fields",
			input:       []byte(`{"auths":{"test.registry.com":{"auth":"` + base64.StdEncoding.EncodeToString([]byte("test:test")) + `"}},"credsStore":"desktop","credHelpers":{"registry.com":"registry-helper"}}`),
			expectError: false,
			description: "valid JSON with auths key and additional docker config fields",
		},
		{
			name:        "invalid JSON - malformed",
			input:       []byte(`{"auths":{"test.registry.com":{"auth":"` + base64.StdEncoding.EncodeToString([]byte("test:test")) + `"}`),
			expectError: true,
			description: "malformed JSON missing closing brace",
		},
		{
			name:        "invalid JSON - trailing comma",
			input:       []byte(`{"auths":{"test.registry.com":{"auth":"` + base64.StdEncoding.EncodeToString([]byte("test:test")) + `"}},}`),
			expectError: true,
			description: "malformed JSON with trailing comma",
		},
		{
			name:        "invalid JSON - unquoted key",
			input:       []byte(`{auths:{"test.registry.com":{"auth":"` + base64.StdEncoding.EncodeToString([]byte("test:test")) + `"}}}`),
			expectError: true,
			description: "malformed JSON with unquoted key",
		},
		{
			name:        "missing auths key",
			input:       []byte(`{"registries":{"test.registry.com":{"auth":"` + base64.StdEncoding.EncodeToString([]byte("test:test")) + `"}}}`),
			expectError: true,
			description: "valid JSON but missing required auths key",
		},
		{
			name:        "empty input",
			input:       []byte(``),
			expectError: true,
			description: "empty byte slice should fail JSON parsing",
		},
		{
			name:        "null input",
			input:       []byte(`null`),
			expectError: true,
			description: "null JSON value should fail validation",
		},
		{
			name:        "string input",
			input:       []byte(`"some string"`),
			expectError: true,
			description: "string JSON value should fail validation",
		},
		{
			name:        "array input",
			input:       []byte(`[{"auths":{"test.registry.com":{"auth":"` + base64.StdEncoding.EncodeToString([]byte("test:test")) + `"}}}]`),
			expectError: true,
			description: "array JSON value should fail validation",
		},
		{
			name:        "number input",
			input:       []byte(`123`),
			expectError: true,
			description: "number JSON value should fail validation",
		},
		{
			name:        "boolean input",
			input:       []byte(`true`),
			expectError: true,
			description: "boolean JSON value should fail validation",
		},
		{
			name:        "auths key with null value",
			input:       []byte(`{"auths":null}`),
			expectError: false,
			description: "auths key with null value should be valid (auths key exists)",
		},
		{
			name:        "auths key with string value",
			input:       []byte(`{"auths":"not an object"}`),
			expectError: false,
			description: "auths key with non-object value should be valid (auths key exists)",
		},
		{
			name:        "auths key with array value",
			input:       []byte(`{"auths":[]}`),
			expectError: false,
			description: "auths key with array value should be valid (auths key exists)",
		},
		{
			name:        "whitespace only",
			input:       []byte(`   `),
			expectError: true,
			description: "whitespace only input should fail JSON parsing",
		},
		{
			name:        "empty object",
			input:       []byte(`{}`),
			expectError: true,
			description: "empty object should fail validation (missing auths key)",
		},
		{
			name:        "nested auths key",
			input:       []byte(`{"config":{"auths":{"test.registry.com":{"auth":"` + base64.StdEncoding.EncodeToString([]byte("test:test")) + `"}}}}`),
			expectError: true,
			description: "auths key nested inside another object should fail validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			err := validateDockerConfigJSON(tt.input)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred(), "Expected error for test case: %s", tt.description)
			} else {
				g.Expect(err).To(BeNil(), "Expected no error for test case: %s, but got: %v", tt.description, err)
			}
		})
	}
}

// generatePullSecretWithAuth generates a pull secret JSON with dynamic base64 auth for a given registry.
// authString should be in format "username:password"
func generatePullSecretWithAuth(registry, authString string) string {
	authBase64 := base64.StdEncoding.EncodeToString([]byte(authString))

	pullSecret := map[string]interface{}{
		"auths": map[string]interface{}{
			registry: map[string]interface{}{
				"auth": authBase64,
			},
		},
	}

	jsonBytes, _ := json.Marshal(pullSecret)
	return string(jsonBytes)
}

// generatePullSecretWithMultipleAuths generates a pull secret JSON with multiple registries and dynamic base64 auth.
// auths is a map of registry -> "username:password"
func generatePullSecretWithMultipleAuths(auths map[string]string) string {
	authsMap := make(map[string]interface{})

	for registry, authString := range auths {
		authBase64 := base64.StdEncoding.EncodeToString([]byte(authString))
		authsMap[registry] = map[string]interface{}{
			"auth": authBase64,
		}
	}

	pullSecret := map[string]interface{}{
		"auths": authsMap,
	}

	jsonBytes, _ := json.Marshal(pullSecret)
	return string(jsonBytes)
}
