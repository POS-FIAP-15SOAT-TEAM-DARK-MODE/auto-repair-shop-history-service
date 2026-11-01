package messaging

import (
	"context"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/env"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// LoadAWSConfig reads AWS_REGION (default us-east-1). Credentials come from the SDK default chain.
func LoadAWSConfig(ctx context.Context) (aws.Config, error) {
	return awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(env.GetString("AWS_REGION", "us-east-1")))
}

// NewSQSClient builds an SQS client; a non-empty endpoint overrides the default (local MiniStack only).
func NewSQSClient(cfg aws.Config, endpoint string) *sqs.Client {
	return sqs.NewFromConfig(cfg, func(o *sqs.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
}
