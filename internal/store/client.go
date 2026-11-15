package store

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// NewDynamoDBClient builds a DynamoDB client. A non-empty endpoint overrides the default (local MiniStack only).
func NewDynamoDBClient(cfg aws.Config, endpoint string) *dynamodb.Client {
	return dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
	})
}
