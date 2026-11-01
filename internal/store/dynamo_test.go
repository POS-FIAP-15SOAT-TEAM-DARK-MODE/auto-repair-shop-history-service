package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	t0      = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	soID    = "01JD8Z4R3Q8M2V7K5N9XWABCDE"
	workID  = "11111111-1111-4111-8111-000000000001"
	workNm  = "Troca de óleo"
	pkValue = "SO#" + soID
)

// writesOf returns the items of the single transaction written by the test.
func writesOf(t *testing.T, api *fakeAPI) []types.TransactWriteItem {
	t.Helper()
	require.Len(t, api.transactions, 1)
	return api.transactions[0].TransactItems
}

func putAttrs(t *testing.T, w types.TransactWriteItem) map[string]types.AttributeValue {
	t.Helper()
	require.NotNil(t, w.Put)
	return w.Put.Item
}

func TestWriteStatusEventWritesDedupeAndItem(t *testing.T) {
	api := &fakeAPI{}
	st := NewDynamoStore(api, "history")

	err := st.WriteStatusEvent(context.Background(), StatusEvent{
		MessageID: "m1", ServiceOrderID: soID, PreviousStatus: "IN_DIAGNOSIS", NewStatus: "AWAITING_APPROVAL",
		Reason: "BUDGET_REJECTED", OccurredAt: t0,
	})
	require.NoError(t, err)

	writes := writesOf(t, api)
	require.Len(t, writes, 2)

	dedupe := writes[0].Put
	require.NotNil(t, dedupe)
	assert.Equal(t, "history", *dedupe.TableName)
	assert.Equal(t, "attribute_not_exists(#pk)", *dedupe.ConditionExpression)
	assert.Equal(t, s("MSG#m1"), dedupe.Item["PK"])
	assert.Equal(t, s("DEDUPE"), dedupe.Item["SK"])

	item := putAttrs(t, writes[1])
	assert.Equal(t, s(pkValue), item["PK"])
	assert.Equal(t, s("S#2026-11-01T12:00:00.000000000Z#m1"), item["SK"])
	assert.Equal(t, s("IN_DIAGNOSIS"), item["previous_status"])
	assert.Equal(t, s("AWAITING_APPROVAL"), item["new_status"])
	assert.Equal(t, s("BUDGET_REJECTED"), item["reason"])
	assert.Equal(t, s("2026-11-01T12:00:00.000000000Z"), item["created_at"])
}

func TestWriteStatusEventOmitsNullPreviousAndReason(t *testing.T) {
	api := &fakeAPI{}
	err := NewDynamoStore(api, "history").WriteStatusEvent(context.Background(), StatusEvent{
		MessageID: "m1", ServiceOrderID: soID, NewStatus: "NEW", OccurredAt: t0,
	})
	require.NoError(t, err)

	item := putAttrs(t, writesOf(t, api)[1])
	assert.NotContains(t, item, "previous_status")
	assert.NotContains(t, item, "reason")
}

func TestWriteStatusEventDuplicateIsReported(t *testing.T) {
	api := &fakeAPI{transactErr: cancelled("ConditionalCheckFailed", "None")}
	err := NewDynamoStore(api, "history").WriteStatusEvent(context.Background(), StatusEvent{
		MessageID: "m1", ServiceOrderID: soID, NewStatus: "NEW", OccurredAt: t0,
	})
	assert.ErrorIs(t, err, ErrDuplicateMessage)
}

func TestTransactionConflictIsAnErrorNotADuplicate(t *testing.T) {
	api := &fakeAPI{transactErr: cancelled("TransactionConflict", "None")}
	err := NewDynamoStore(api, "history").WriteStatusEvent(context.Background(), StatusEvent{
		MessageID: "m1", ServiceOrderID: soID, NewStatus: "NEW", OccurredAt: t0,
	})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrDuplicateMessage))
}

func TestTransactionErrorWithoutReasonsIsAnError(t *testing.T) {
	api := &fakeAPI{transactErr: &types.TransactionCanceledException{}}
	err := NewDynamoStore(api, "history").WriteStatusEvent(context.Background(), StatusEvent{
		MessageID: "m1", ServiceOrderID: soID, NewStatus: "NEW", OccurredAt: t0,
	})
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrDuplicateMessage))
}

func TestGenericTransactErrorIsWrapped(t *testing.T) {
	api := &fakeAPI{transactErr: errBoom}
	err := NewDynamoStore(api, "history").WriteStatusEvent(context.Background(), StatusEvent{
		MessageID: "m1", ServiceOrderID: soID, NewStatus: "NEW", OccurredAt: t0,
	})
	assert.ErrorIs(t, err, errBoom)
}

func TestWriteWorkEventWithoutSpecialTransitionWritesOnlyTheItem(t *testing.T) {
	api := &fakeAPI{}
	err := NewDynamoStore(api, "history").WriteWorkEvent(context.Background(), WorkEvent{
		MessageID: "m2", ServiceOrderID: soID, WorkID: workID, WorkName: workNm,
		NewStatus: "AWAITING_START", OccurredAt: t0,
	})
	require.NoError(t, err)

	writes := writesOf(t, api)
	require.Len(t, writes, 2)
	item := putAttrs(t, writes[1])
	assert.Equal(t, s("W#"+workID+"#2026-11-01T12:00:00.000000000Z#m2"), item["SK"])
	assert.Equal(t, s(workID), item["work_id"])
	assert.Equal(t, s(workNm), item["work_name"])
	assert.Equal(t, s("AWAITING_START"), item["new_status"])
	assert.NotContains(t, item, "previous_status")
	assert.Empty(t, api.queryCalls)
}

func TestWriteWorkEventInProgressWritesStartMarker(t *testing.T) {
	api := &fakeAPI{}
	err := NewDynamoStore(api, "history").WriteWorkEvent(context.Background(), WorkEvent{
		MessageID: "m3", ServiceOrderID: soID, WorkID: workID, WorkName: workNm,
		PreviousStatus: "AWAITING_START", NewStatus: "IN_PROGRESS", OccurredAt: t0,
	})
	require.NoError(t, err)

	writes := writesOf(t, api)
	require.Len(t, writes, 3)
	item := putAttrs(t, writes[1])
	assert.Equal(t, s("AWAITING_START"), item["previous_status"])

	marker := putAttrs(t, writes[2])
	assert.Equal(t, s(pkValue), marker["PK"])
	assert.Equal(t, s("R#"+workID), marker["SK"])
	assert.Equal(t, s("2026-11-01T12:00:00.000000000Z"), marker["started_at"])
}

func TestWriteWorkEventCompletedAddsDurationToReport(t *testing.T) {
	started := t0.Add(-150 * time.Minute)
	api := &fakeAPI{getOut: &dynamodb.GetItemOutput{Item: map[string]types.AttributeValue{
		"PK": s(pkValue), "SK": s("R#" + workID), "started_at": s(started.Format(time.RFC3339Nano)),
	}}}

	err := NewDynamoStore(api, "history").WriteWorkEvent(context.Background(), WorkEvent{
		MessageID: "m4", ServiceOrderID: soID, WorkID: workID, WorkName: workNm,
		PreviousStatus: "IN_PROGRESS", NewStatus: "COMPLETED", OccurredAt: t0,
	})
	require.NoError(t, err)

	writes := writesOf(t, api)
	require.Len(t, writes, 3)
	upd := writes[2].Update
	require.NotNil(t, upd)
	assert.Equal(t, s("REPORT#WORK"), upd.Key["PK"])
	assert.Equal(t, s("W#"+workID), upd.Key["SK"])
	assert.Equal(t, "ADD #count :one, #total :seconds SET #name = :name", *upd.UpdateExpression)
	assert.Equal(t, n("1"), upd.ExpressionAttributeValues[":one"])
	assert.Equal(t, n("9000"), upd.ExpressionAttributeValues[":seconds"]) // 2.5 hours
	assert.Equal(t, s(workNm), upd.ExpressionAttributeValues[":name"])
}

func TestWriteWorkEventCompletedWithoutMarkerSkipsReport(t *testing.T) {
	api := &fakeAPI{}
	err := NewDynamoStore(api, "history").WriteWorkEvent(context.Background(), WorkEvent{
		MessageID: "m5", ServiceOrderID: soID, WorkID: workID, WorkName: workNm,
		PreviousStatus: "IN_PROGRESS", NewStatus: "COMPLETED", OccurredAt: t0,
	})
	require.NoError(t, err)
	writes := writesOf(t, api)
	assert.Len(t, writes, 2, "only dedupe and the transition item when the start marker is missing")
	assert.Nil(t, writes[len(writes)-1].Update)
}

func TestWriteWorkEventCompletedMarkerReadErrorStopsWrite(t *testing.T) {
	api := &fakeAPI{getErr: errBoom}
	err := NewDynamoStore(api, "history").WriteWorkEvent(context.Background(), WorkEvent{
		MessageID: "m6", ServiceOrderID: soID, WorkID: workID, WorkName: workNm,
		PreviousStatus: "IN_PROGRESS", NewStatus: "COMPLETED", OccurredAt: t0,
	})
	assert.ErrorIs(t, err, errBoom)
	assert.Empty(t, api.transactions, "no write when the marker cannot be read")
}

func TestWriteWorkEventCompletedMalformedMarkerIsAnError(t *testing.T) {
	cases := map[string]map[string]types.AttributeValue{
		"not a string": {"started_at": n("1")},
		"not a time":   {"started_at": s("yesterday")},
	}
	for name, item := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{getOut: &dynamodb.GetItemOutput{Item: item}}
			err := NewDynamoStore(api, "history").WriteWorkEvent(context.Background(), WorkEvent{
				MessageID: "m7", ServiceOrderID: soID, WorkID: workID, WorkName: workNm,
				PreviousStatus: "IN_PROGRESS", NewStatus: "COMPLETED", OccurredAt: t0,
			})
			assert.Error(t, err)
			assert.Empty(t, api.transactions)
		})
	}
}

func TestWriteWorkEventDuplicateIsReported(t *testing.T) {
	api := &fakeAPI{transactErr: cancelled("ConditionalCheckFailed")}
	err := NewDynamoStore(api, "history").WriteWorkEvent(context.Background(), WorkEvent{
		MessageID: "m8", ServiceOrderID: soID, WorkID: workID, WorkName: workNm,
		NewStatus: "AWAITING_START", OccurredAt: t0,
	})
	assert.ErrorIs(t, err, ErrDuplicateMessage)
}

func TestNewDynamoDBClientOverridesEndpointOnlyWhenSet(t *testing.T) {
	cfg := aws.Config{Region: "us-east-1"}
	withEndpoint := NewDynamoDBClient(cfg, "http://localhost:4566")
	assert.Equal(t, "http://localhost:4566", aws.ToString(withEndpoint.Options().BaseEndpoint))

	withoutEndpoint := NewDynamoDBClient(cfg, "")
	assert.Nil(t, withoutEndpoint.Options().BaseEndpoint)
}
