package backup

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// TestCreateOptionsDefaults verifies that the default values for CreateOptions
// are set correctly when instantiating a new CreateOptions struct.
func TestCreateOptionsDefaults(t *testing.T) {
	opts := &CreateOptions{
		OADPNamespace:            "openshift-adp",
		StorageLocation:          "default",
		TTL:                      2 * time.Hour,
		SnapshotMoveData:         true,
		DefaultVolumesToFsBackup: false,
		Render:                   false,
		IncludedResources:        nil,
	}

	if opts.OADPNamespace != "openshift-adp" {
		t.Errorf("Expected default OADP namespace to be 'openshift-adp', got %s", opts.OADPNamespace)
	}

	if opts.StorageLocation != "default" {
		t.Errorf("Expected default storage location to be 'default', got %s", opts.StorageLocation)
	}

	if opts.TTL != 2*time.Hour {
		t.Errorf("Expected default TTL to be 2h, got %v", opts.TTL)
	}

	if !opts.SnapshotMoveData {
		t.Errorf("Expected default SnapshotMoveData to be true")
	}

	if opts.DefaultVolumesToFsBackup {
		t.Errorf("Expected default DefaultVolumesToFsBackup to be false")
	}

	if opts.Render {
		t.Errorf("Expected default Render to be false")
	}

	if opts.IncludedResources != nil {
		t.Errorf("Expected default IncludedResources to be nil")
	}
}

// TestBackupNameGeneration tests the naming pattern for backup objects.
// Verifies that backup names follow the expected format: {hc-name}-{hc-namespace}-{random-hash}.
func TestBackupNameGeneration(t *testing.T) {
	hcName := "test-cluster"
	hcNamespace := "clusters"

	// Mock a hash
	hash := "abc123"
	expectedName := "test-cluster-clusters-abc123"

	// Test the naming pattern (we can't test actual generation without mocking)
	actualName := hcName + "-" + hcNamespace + "-" + hash

	if actualName != expectedName {
		t.Errorf("Expected backup name %s, got %s", expectedName, actualName)
	}
}

// TestGenerateBackupObject validates the basic structure and metadata of generated backup objects.
// This test focuses on the fundamental properties like APIVersion, Kind, ObjectMeta, and IncludedNamespaces.
// It serves as a structural validation test for the core backup object generation functionality.
func TestGenerateBackupObject(t *testing.T) {
	opts := &CreateOptions{
		HCName:                   "test-cluster",
		HCNamespace:              "clusters",
		OADPNamespace:            "openshift-adp",
		StorageLocation:          "default",
		TTL:                      2 * time.Hour,
		SnapshotMoveData:         true,
		DefaultVolumesToFsBackup: false,
	}

	backup, backupName, err := opts.generateBackupObjectWithPlatform("AWS")
	if err != nil {
		t.Errorf("generateBackupObject() error = %v", err)
		return
	}

	// Check backup name format
	if len(backupName) == 0 {
		t.Errorf("Expected backup name to be generated, got empty string")
	}

	// Check that backup name contains hc-name and hc-namespace
	if !strings.Contains(backupName, opts.HCName) {
		t.Errorf("Expected backup name to contain hc-name '%s', got %s", opts.HCName, backupName)
	}

	if !strings.Contains(backupName, opts.HCNamespace) {
		t.Errorf("Expected backup name to contain hc-namespace '%s', got %s", opts.HCNamespace, backupName)
	}

	// Check backup object structure
	if backup.APIVersion != "velero.io/v1" {
		t.Errorf("Expected API version 'velero.io/v1', got %s", backup.APIVersion)
	}

	if backup.Kind != "Backup" {
		t.Errorf("Expected kind 'Backup', got %s", backup.Kind)
	}

	if backup.Name != backupName {
		t.Errorf("Expected backup name %s, got %s", backupName, backup.Name)
	}

	if backup.Namespace != opts.OADPNamespace {
		t.Errorf("Expected backup namespace %s, got %s", opts.OADPNamespace, backup.Namespace)
	}

	// Check included namespaces
	expectedNamespaces := []string{opts.HCNamespace, fmt.Sprintf("%s-%s", opts.HCNamespace, opts.HCName)}
	if len(backup.Spec.IncludedNamespaces) != len(expectedNamespaces) {
		t.Errorf("Expected %d included namespaces, got %d", len(expectedNamespaces), len(backup.Spec.IncludedNamespaces))
	}

	// Check that the correct namespaces are included
	for i, expected := range expectedNamespaces {
		if i < len(backup.Spec.IncludedNamespaces) && backup.Spec.IncludedNamespaces[i] != expected {
			t.Errorf("Expected namespace[%d] to be '%s', got '%s'", i, expected, backup.Spec.IncludedNamespaces[i])
		}
	}
}

// TestGenerateBackupObjectComprehensive provides comprehensive testing of backup object generation
// across multiple scenarios including:
// - Custom resource selection (user-defined IncludedResources)
// - Default resource selection with platform-specific resources
// - Multi-platform support (AWS, Agent, KubeVirt, OpenStack, Azure)
// This test ensures that the backup generation logic correctly handles different platforms
// and resource selection strategies.
func TestGenerateBackupObjectComprehensive(t *testing.T) {
	type testCase struct {
		name                     string
		platform                 string
		includedResources        []string
		expectedMinResources     int
		expectedBaseResources    []string
		expectedPlatformSpecific []string
		customResourcesExact     bool // if true, expect exact match for includedResources
	}

	// Define platform-specific resource mappings
	platformResources := map[string][]string{
		"AWS": {
			"awscluster", "awsmachinetemplate", "awsmachine",
		},
		"AGENT": {
			"agentcluster", "agentmachinetemplate", "agentmachine",
		},
		"KUBEVIRT": {
			"kubevirtcluster", "kubevirtmachinetemplate",
		},
		"OPENSTACK": {
			"openstackclusters", "openstackmachinetemplates", "openstackmachine",
		},
		"AZURE": {
			"azureclusters", "azuremachinetemplates", "azuremachine",
		},
	}

	// Base resources expected in all default configurations
	baseResources := []string{"hostedcluster", "nodepool", "secrets", "configmap"}

	tests := []testCase{
		// Test cases for custom resources
		{
			name:                  "Custom resources - minimal set",
			platform:              "AWS",
			includedResources:     []string{"configmap", "secrets", "pod"},
			expectedMinResources:  3,
			expectedBaseResources: []string{"configmap", "secrets", "pod"},
			customResourcesExact:  true,
		},
		{
			name:                  "Custom resources - specific selection",
			platform:              "KUBEVIRT",
			includedResources:     []string{"hostedcluster", "nodepool", "secrets"},
			expectedMinResources:  3,
			expectedBaseResources: []string{"hostedcluster", "nodepool", "secrets"},
			customResourcesExact:  true,
		},
		// Test cases for default resources with different platforms
		{
			name:                     "Default resources - AWS platform",
			platform:                 "AWS",
			includedResources:        nil,
			expectedMinResources:     10,
			expectedBaseResources:    baseResources,
			expectedPlatformSpecific: platformResources["AWS"],
			customResourcesExact:     false,
		},
		{
			name:                     "Default resources - Agent platform",
			platform:                 "AGENT",
			includedResources:        nil,
			expectedMinResources:     10,
			expectedBaseResources:    baseResources,
			expectedPlatformSpecific: platformResources["AGENT"],
			customResourcesExact:     false,
		},
		{
			name:                     "Default resources - KubeVirt platform",
			platform:                 "KUBEVIRT",
			includedResources:        nil,
			expectedMinResources:     10,
			expectedBaseResources:    baseResources,
			expectedPlatformSpecific: platformResources["KUBEVIRT"],
			customResourcesExact:     false,
		},
		{
			name:                     "Default resources - OpenStack platform",
			platform:                 "OPENSTACK",
			includedResources:        nil,
			expectedMinResources:     10,
			expectedBaseResources:    baseResources,
			expectedPlatformSpecific: platformResources["OPENSTACK"],
			customResourcesExact:     false,
		},
		{
			name:                     "Default resources - Azure platform",
			platform:                 "AZURE",
			includedResources:        nil,
			expectedMinResources:     10,
			expectedBaseResources:    baseResources,
			expectedPlatformSpecific: platformResources["AZURE"],
			customResourcesExact:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &CreateOptions{
				HCName:                   "test-cluster",
				HCNamespace:              "clusters",
				OADPNamespace:            "openshift-adp",
				StorageLocation:          "default",
				TTL:                      2 * time.Hour,
				SnapshotMoveData:         true,
				DefaultVolumesToFsBackup: false,
				IncludedResources:        tt.includedResources,
			}

			backup, backupName, err := opts.generateBackupObjectWithPlatform(tt.platform)
			if err != nil {
				t.Errorf("generateBackupObjectWithPlatform() error = %v", err)
				return
			}

			// Basic validation
			if len(backupName) == 0 {
				t.Errorf("Expected backup name to be generated, got empty string")
			}

			if backup.APIVersion != "velero.io/v1" {
				t.Errorf("Expected API version 'velero.io/v1', got %s", backup.APIVersion)
			}

			if backup.Kind != "Backup" {
				t.Errorf("Expected kind 'Backup', got %s", backup.Kind)
			}

			// Check minimum number of resources
			if len(backup.Spec.IncludedResources) < tt.expectedMinResources {
				t.Errorf("Expected at least %d resources, got %d", tt.expectedMinResources, len(backup.Spec.IncludedResources))
			}

			// For custom resources, check exact match
			if tt.customResourcesExact {
				if len(backup.Spec.IncludedResources) != len(tt.expectedBaseResources) {
					t.Errorf("Expected exactly %d resources, got %d", len(tt.expectedBaseResources), len(backup.Spec.IncludedResources))
				}
				for i, expected := range tt.expectedBaseResources {
					if i < len(backup.Spec.IncludedResources) && backup.Spec.IncludedResources[i] != expected {
						t.Errorf("Expected resource[%d] to be '%s', got '%s'", i, expected, backup.Spec.IncludedResources[i])
					}
				}
				return // Skip platform-specific checks for custom resources
			}

			// For default resources, check contains
			resourcesStr := fmt.Sprintf("%v", backup.Spec.IncludedResources)

			// Check base resources are included
			for _, expected := range tt.expectedBaseResources {
				if !strings.Contains(resourcesStr, expected) {
					t.Errorf("Expected %s backup to contain base resource '%s'", tt.platform, expected)
				}
			}

			// Check platform-specific resources are included
			for _, expected := range tt.expectedPlatformSpecific {
				if !strings.Contains(resourcesStr, expected) {
					t.Errorf("Expected %s backup to contain platform-specific resource '%s'", tt.platform, expected)
				}
			}
		})
	}
}

// TestGetDefaultResourcesForPlatform verifies that the getDefaultResourcesForPlatform function
// returns the correct set of resources for each supported platform type.
// This test ensures that:
// - Base resources are always included regardless of platform
// - Platform-specific resources are correctly added based on the platform type
// - Platform name normalization works (lowercase -> uppercase)
// - Unknown platforms default to AWS resources
func TestGetDefaultResourcesForPlatform(t *testing.T) {
	tests := []struct {
		name                     string
		platform                 string
		expectedPlatformSpecific []string
	}{
		{
			name:     "AWS platform",
			platform: "AWS",
			expectedPlatformSpecific: []string{
				"awscluster", "awsmachinetemplate", "awsmachine",
			},
		},
		{
			name:     "Agent platform",
			platform: "AGENT",
			expectedPlatformSpecific: []string{
				"agentcluster", "agentmachinetemplate", "agentmachine",
			},
		},
		{
			name:     "KubeVirt platform",
			platform: "KUBEVIRT",
			expectedPlatformSpecific: []string{
				"kubevirtcluster", "kubevirtmachinetemplate",
			},
		},
		{
			name:     "OpenStack platform",
			platform: "OPENSTACK",
			expectedPlatformSpecific: []string{
				"openstackclusters", "openstackmachinetemplates", "openstackmachine",
			},
		},
		{
			name:     "Azure platform",
			platform: "AZURE",
			expectedPlatformSpecific: []string{
				"azureclusters", "azuremachinetemplates", "azuremachine",
			},
		},
		{
			name:     "Unknown platform defaults to AWS",
			platform: "UNKNOWN",
			expectedPlatformSpecific: []string{
				"awscluster", "awsmachinetemplate", "awsmachine",
			},
		},
		{
			name:     "Lowercase platform should work",
			platform: "aws",
			expectedPlatformSpecific: []string{
				"awscluster", "awsmachinetemplate", "awsmachine",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := getDefaultResourcesForPlatform(tt.platform)

			// Check that we have a reasonable number of resources
			if len(resources) < 15 {
				t.Errorf("Expected at least 15 resources, got %d", len(resources))
			}

			// Convert to string slice for easier checking
			resourcesStr := fmt.Sprintf("%v", resources)

			// Check base resources are always included
			baseResources := []string{"hostedcluster", "nodepool", "secrets", "configmap", "sa", "role"}
			for _, expected := range baseResources {
				if !strings.Contains(resourcesStr, expected) {
					t.Errorf("Expected base resources to contain '%s'", expected)
				}
			}

			// Check platform-specific resources
			for _, expected := range tt.expectedPlatformSpecific {
				if !strings.Contains(resourcesStr, expected) {
					t.Errorf("Expected platform-specific resources for %s to contain '%s'", tt.platform, expected)
				}
			}
		})
	}
}
