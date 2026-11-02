package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/logger"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"go.uber.org/zap"
)

// conditionalCheckFailed is the cancellation code DynamoDB returns when a condition of a transaction fails.
const conditionalCheckFailed = "ConditionalCheckFailed"

// DynamoAPI is the subset of the DynamoDB client used by the store. *dynamodb.Client satisfies it.
type DynamoAPI interface {
	TransactWriteItems(ctx context.Context, params *dynamodb.TransactWriteItemsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
}

// DynamoStore implements Store on a single DynamoDB table with PK/SK keys.
type DynamoStore struct {
	api   DynamoAPI
	table string
}

// NewDynamoStore returns a Store on the given table.
func NewDynamoStore(api DynamoAPI, table string) *DynamoStore {
	return &DynamoStore{api: api, table: table}
}

// WriteStatusEvent writes the dedupe marker and the transition in one transaction.
func (s *DynamoStore) WriteStatusEvent(ctx context.Context, e StatusEvent) error {
	item := map[string]types.AttributeValue{
		attrPK:       sAttr(soPK(e.ServiceOrderID)),
		attrSK:       sAttr(statusSK(e.OccurredAt, e.MessageID)),
		"new_status": sAttr(e.NewStatus),
		"created_at": sAttr(formatTime(e.OccurredAt)),
	}
	if e.PreviousStatus != "" {
		item["previous_status"] = sAttr(e.PreviousStatus)
	}
	if e.Reason != "" {
		item["reason"] = sAttr(e.Reason)
	}
	return s.transact(ctx, e.MessageID, e.OccurredAt, putItem(s.table, item))
}

// WriteWorkEvent writes the transition. In IN_PROGRESS it also writes the start marker; in IN_PROGRESS -> COMPLETED
// it adds the duration to the report aggregate when the marker exists. All writes share one transaction.
func (s *DynamoStore) WriteWorkEvent(ctx context.Context, e WorkEvent) error {
	pk := soPK(e.ServiceOrderID)
	item := map[string]types.AttributeValue{
		attrPK:       sAttr(pk),
		attrSK:       sAttr(workSK(e.WorkID, e.OccurredAt, e.MessageID)),
		"work_id":    sAttr(e.WorkID),
		"work_name":  sAttr(e.WorkName),
		"new_status": sAttr(e.NewStatus),
		"created_at": sAttr(formatTime(e.OccurredAt)),
	}
	if e.PreviousStatus != "" {
		item["previous_status"] = sAttr(e.PreviousStatus)
	}
	writes := []types.TransactWriteItem{putItem(s.table, item)}

	switch {
	case e.NewStatus == WorkStatusInProgress:
		writes = append(writes, putItem(s.table, map[string]types.AttributeValue{
			attrPK:       sAttr(pk),
			attrSK:       sAttr(startedSK(e.WorkID)),
			"started_at": sAttr(formatTime(e.OccurredAt)),
		}))
	case e.PreviousStatus == WorkStatusInProgress && e.NewStatus == WorkStatusCompleted:
		startedAt, found, err := s.startedAt(ctx, pk, e.WorkID)
		if err != nil {
			return err
		}
		if !found {
			logger.Of(ctx).Warn("work.start_marker_missing",
				zap.String("work_id", e.WorkID), zap.String("message_id", e.MessageID))
			break
		}
		writes = append(writes, s.reportUpdate(e.WorkID, e.WorkName, e.OccurredAt.Sub(startedAt).Seconds()))
	}

	return s.transact(ctx, e.MessageID, e.OccurredAt, writes...)
}

// startedAt reads the start marker with a consistent read.
func (s *DynamoStore) startedAt(ctx context.Context, pk, workID string) (time.Time, bool, error) {
	out, err := s.api.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(s.table),
		Key:            key(pk, startedSK(workID)),
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return time.Time{}, false, fmt.Errorf("get start marker: %w", err)
	}
	if len(out.Item) == 0 {
		return time.Time{}, false, nil
	}
	raw, err := stringAttr(out.Item, "started_at")
	if err != nil {
		return time.Time{}, false, err
	}
	startedAt, err := parseTime(raw)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse started_at: %w", err)
	}
	return startedAt, true, nil
}

func (s *DynamoStore) reportUpdate(workID, workName string, seconds float64) types.TransactWriteItem {
	return types.TransactWriteItem{Update: &types.Update{
		TableName:        aws.String(s.table),
		Key:              key(reportPK, reportSK(workID)),
		UpdateExpression: aws.String("ADD #count :one, #total :seconds SET #name = :name"),
		ExpressionAttributeNames: map[string]string{
			"#count": "count",
			"#total": "total_seconds",
			"#name":  "work_name",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one":     nAttr("1"),
			":seconds": nAttr(formatSeconds(seconds)),
			":name":    sAttr(workName),
		},
	}}
}

// transact writes the dedupe marker first, then the given writes, atomically. A failed dedupe condition is a
// duplicate message; any other failure is returned as an error so the message stays in the queue.
func (s *DynamoStore) transact(ctx context.Context, messageID string, occurredAt time.Time, writes ...types.TransactWriteItem) error {
	dedupe := types.TransactWriteItem{Put: &types.Put{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			attrPK:         sAttr(dedupePK(messageID)),
			attrSK:         sAttr(dedupeSK),
			"processed_at": sAttr(formatTime(occurredAt)),
		},
		ConditionExpression:      aws.String("attribute_not_exists(#pk)"),
		ExpressionAttributeNames: map[string]string{"#pk": attrPK},
	}}

	_, err := s.api.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: append([]types.TransactWriteItem{dedupe}, writes...),
	})
	if err == nil {
		return nil
	}

	var canceled *types.TransactionCanceledException
	if errors.As(err, &canceled) && len(canceled.CancellationReasons) > 0 &&
		aws.ToString(canceled.CancellationReasons[0].Code) == conditionalCheckFailed {
		return ErrDuplicateMessage
	}
	return fmt.Errorf("transact write: %w", err)
}

func putItem(table string, item map[string]types.AttributeValue) types.TransactWriteItem {
	return types.TransactWriteItem{Put: &types.Put{TableName: aws.String(table), Item: item}}
}

func key(pk, sk string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{attrPK: sAttr(pk), attrSK: sAttr(sk)}
}

func sAttr(v string) types.AttributeValue {
	return &types.AttributeValueMemberS{Value: v}
}

func nAttr(v string) types.AttributeValue {
	return &types.AttributeValueMemberN{Value: v}
}

func stringAttr(item map[string]types.AttributeValue, name string) (string, error) {
	v, ok := item[name].(*types.AttributeValueMemberS)
	if !ok {
		return "", fmt.Errorf("attribute %s: not a string", name)
	}
	return v.Value, nil
}

func optionalString(item map[string]types.AttributeValue, name string) string {
	v, ok := item[name].(*types.AttributeValueMemberS)
	if !ok {
		return ""
	}
	return v.Value
}

func numberAttr(item map[string]types.AttributeValue, name string) (string, error) {
	v, ok := item[name].(*types.AttributeValueMemberN)
	if !ok {
		return "", fmt.Errorf("attribute %s: not a number", name)
	}
	return v.Value, nil
}
