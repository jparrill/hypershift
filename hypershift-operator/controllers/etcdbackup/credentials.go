package etcdbackup

import (
	"encoding/json"
	"strings"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"

	corev1 "k8s.io/api/core/v1"
)

type credentialMode string

const (
	credentialModeAWSStatic             credentialMode = "aws-static"
	credentialModeAWSSTS                credentialMode = "aws-sts"
	credentialModeAzureClientSecret     credentialMode = "azure-client-secret"
	credentialModeAzureWorkloadIdentity credentialMode = "azure-workload-identity"
	credentialModeAzureManagedIdentity  credentialMode = "azure-managed-identity"
)

type resolvedCredentials struct {
	Mode       credentialMode
	SecretName string
	RoleARN    string
	ClientID   string
}

func (c resolvedCredentials) needsCredentialsFile() bool {
	switch c.Mode {
	case credentialModeAWSStatic, credentialModeAzureClientSecret, credentialModeAzureManagedIdentity:
		return true
	default:
		return false
	}
}

func resolveCredentials(storageType hyperv1.HCPEtcdBackupStorageType, secret *corev1.Secret) resolvedCredentials {
	switch storageType {
	case hyperv1.S3BackupStorage:
		return resolveAWSCredentials(secret)
	case hyperv1.AzureBlobBackupStorage:
		return resolveAzureCredentials(secret)
	default:
		return resolvedCredentials{Mode: credentialModeAWSStatic, SecretName: secret.Name}
	}
}

func resolveAWSCredentials(secret *corev1.Secret) resolvedCredentials {
	creds := strings.TrimSpace(string(secret.Data["credentials"]))
	if strings.HasPrefix(creds, "arn:") {
		return resolvedCredentials{
			Mode:       credentialModeAWSSTS,
			SecretName: secret.Name,
			RoleARN:    creds,
		}
	}
	return resolvedCredentials{
		Mode:       credentialModeAWSStatic,
		SecretName: secret.Name,
	}
}

func resolveAzureCredentials(secret *corev1.Secret) resolvedCredentials {
	if cloudData, ok := secret.Data["cloud"]; ok {
		var clientID string
		for line := range strings.SplitSeq(string(cloudData), "\n") {
			line = strings.TrimSpace(line)
			if v, ok := strings.CutPrefix(line, "AZURE_CLIENT_ID="); ok {
				clientID = v
				break
			}
		}
		if clientID == "" {
			return resolvedCredentials{
				Mode:       credentialModeAzureManagedIdentity,
				SecretName: secret.Name,
			}
		}
		return resolvedCredentials{
			Mode:       credentialModeAzureWorkloadIdentity,
			SecretName: secret.Name,
			ClientID:   clientID,
		}
	}

	if credData, ok := secret.Data["credentials"]; ok {
		var creds struct {
			ClientSecret string `json:"clientSecret"`
		}
		if err := json.Unmarshal(credData, &creds); err == nil && creds.ClientSecret != "" {
			return resolvedCredentials{
				Mode:       credentialModeAzureClientSecret,
				SecretName: secret.Name,
			}
		}
	}

	return resolvedCredentials{
		Mode:       credentialModeAzureManagedIdentity,
		SecretName: secret.Name,
	}
}
