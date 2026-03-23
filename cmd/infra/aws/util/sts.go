package util

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	supportawsutil "github.com/openshift/hypershift/support/awsutil"

	awsv2 "github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	configv2 "github.com/aws/aws-sdk-go-v2/config"
	v2credentials "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/smithy-go/middleware"
)

func NewSTSSessionV2(ctx context.Context, agent, roleArn, region string, assumeRoleCreds *awsv2.Credentials) (*awsv2.Config, error) {
	if assumeRoleCreds == nil {
		return nil, fmt.Errorf("assumeRoleCreds cannot be nil")
	}
	if roleArn == "" {
		return nil, fmt.Errorf("roleArn cannot be empty")
	}
	if agent == "" {
		return nil, fmt.Errorf("agent cannot be empty")
	}

	cfg, err := configv2.LoadDefaultConfig(ctx,
		configv2.WithCredentialsProvider(v2credentials.StaticCredentialsProvider{
			Value: *assumeRoleCreds,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load v2 config: %w", err)
	}

	assumedCreds, err := supportawsutil.AssumeRole(ctx, cfg, agent, roleArn)
	if err != nil {
		return nil, err
	}

	var loadOptions []func(*configv2.LoadOptions) error
	loadOptions = append(loadOptions, configv2.WithCredentialsProvider(
		v2credentials.StaticCredentialsProvider{
			Value: *assumedCreds,
		},
	))
	if region != "" {
		loadOptions = append(loadOptions, configv2.WithRegion(region))
	}
	loadOptions = append(loadOptions, configv2.WithAPIOptions([]func(*middleware.Stack) error{
		awsmiddleware.AddUserAgentKeyValue("openshift.io hypershift", agent),
	}))
	finalCfg, err := configv2.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to load final config: %w", err)
	}

	return &finalCfg, nil
}

type STSCreds struct {
	Credentials Credentials `json:"Credentials"`
}

// ParseSTSCredentialsFileV2 parses STS credentials file and returns v2 credentials
func ParseSTSCredentialsFileV2(credentialsFile string) (*awsv2.Credentials, error) {
	var stsCreds STSCreds

	rawSTSCreds, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read sts credentials file: %w", err)
	}
	err = json.Unmarshal(rawSTSCreds, &stsCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal sts credentials: %w", err)
	}

	creds := awsv2.Credentials{
		AccessKeyID:     stsCreds.Credentials.AccessKeyId,
		SecretAccessKey: stsCreds.Credentials.SecretAccessKey,
		SessionToken:    stsCreds.Credentials.SessionToken,
	}
	return &creds, nil
}

type Credentials struct {
	AccessKeyId     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	SessionToken    string `json:"SessionToken"`
	Expiration      string `json:"Expiration"`
}
