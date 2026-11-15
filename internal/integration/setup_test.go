//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/messaging"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// newTestStore creates a fresh table on the endpoint in AWS_ENDPOINT_URL (MiniStack in CI) and returns a store on it.
func newTestStore(t *testing.T) (*store.DynamoStore, *dynamodb.Client, string) {
	t.Helper()
	ctx := context.Background()

	cfg, err := messaging.LoadAWSConfig(ctx)
	if err != nil {
		t.Fatalf("load aws config: %v", err)
	}
	client := store.NewDynamoDBClient(cfg, os.Getenv("AWS_ENDPOINT_URL"))

	table := fmt.Sprintf("auto-repair-shop-it-history-%s", uuid.NewString())
	_, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName:   aws.String(table),
		BillingMode: types.BillingModePayPerRequest,
		AttributeDefinitions: []types.AttributeDefinition{
			{AttributeName: aws.String("PK"), AttributeType: types.ScalarAttributeTypeS},
			{AttributeName: aws.String("SK"), AttributeType: types.ScalarAttributeTypeS},
		},
		KeySchema: []types.KeySchemaElement{
			{AttributeName: aws.String("PK"), KeyType: types.KeyTypeHash},
			{AttributeName: aws.String("SK"), KeyType: types.KeyTypeRange},
		},
	})
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	t.Cleanup(func() {
		_, _ = client.DeleteTable(context.Background(), &dynamodb.DeleteTableInput{TableName: aws.String(table)})
	})

	return store.NewDynamoStore(client, table), client, table
}
