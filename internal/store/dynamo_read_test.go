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

func statusItem(prev, next, reason, createdAt string) map[string]types.AttributeValue {
	item := map[string]types.AttributeValue{
		"PK":         s(pkValue),
		"SK":         s("S#" + createdAt + "#m"),
		"new_status": s(next),
		"created_at": s(createdAt),
	}
	if prev != "" {
		item["previous_status"] = s(prev)
	}
	if reason != "" {
		item["reason"] = s(reason)
	}
	return item
}

func workItem(id, name, prev, next, createdAt string) map[string]types.AttributeValue {
	item := map[string]types.AttributeValue{
		"PK":         s(pkValue),
		"SK":         s("W#" + id + "#" + createdAt + "#m"),
		"work_id":    s(id),
		"work_name":  s(name),
		"new_status": s(next),
		"created_at": s(createdAt),
	}
	if prev != "" {
		item["previous_status"] = s(prev)
	}
	return item
}

// routeQueries answers Query calls by the sort-key prefix they ask for.
func routeQueries(byPrefix map[string][]map[string]types.AttributeValue) func(*dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
	return func(in *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
		prefix := in.ExpressionAttributeValues[":prefix"].(*types.AttributeValueMemberS).Value
		return &dynamodb.QueryOutput{Items: byPrefix[prefix]}, nil
	}
}

func TestListByServiceOrderDecodesBothKinds(t *testing.T) {
	api := &fakeAPI{queryFn: routeQueries(map[string][]map[string]types.AttributeValue{
		"S#": {
			statusItem("", "NEW", "", "2026-11-01T11:59:00.000000000Z"),
			statusItem("NEW", "IN_PROGRESS", "", "2026-11-01T12:00:00.000000000Z"),
			statusItem("IN_PROGRESS", "REJECTED", "PAYMENT_REJECTED", "2026-11-01T13:00:00.000000000Z"),
		},
		"W#": {
			workItem(workID, workNm, "", "AWAITING_START", "2026-11-01T11:59:30.000000000Z"),
			workItem(workID, workNm, "AWAITING_START", "IN_PROGRESS", "2026-11-01T12:00:00.000000000Z"),
		},
	})}
	st := NewDynamoStore(api, "history")

	statuses, works, err := st.ListByServiceOrder(context.Background(), soID)
	require.NoError(t, err)

	require.Len(t, statuses, 3)
	assert.Equal(t, "", statuses[0].PreviousStatus)
	assert.Equal(t, "NEW", statuses[0].NewStatus)
	assert.Equal(t, "NEW", statuses[1].PreviousStatus)
	assert.Equal(t, "PAYMENT_REJECTED", statuses[2].Reason)
	assert.Equal(t, time.Date(2026, 11, 1, 13, 0, 0, 0, time.UTC), statuses[2].CreatedAt)

	require.Len(t, works, 2)
	assert.Equal(t, workID, works[0].WorkID)
	assert.Equal(t, workNm, works[1].WorkName)
	assert.Equal(t, "AWAITING_START", works[1].PreviousStatus)

	require.Len(t, api.queryCalls, 2)
	assert.True(t, aws.ToBool(api.queryCalls[0].ConsistentRead))
	assert.Equal(t, "history", aws.ToString(api.queryCalls[0].TableName))
}

func TestListByServiceOrderFollowsPagination(t *testing.T) {
	page := 0
	api := &fakeAPI{queryFn: func(in *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
		prefix := in.ExpressionAttributeValues[":prefix"].(*types.AttributeValueMemberS).Value
		if prefix != "S#" {
			return &dynamodb.QueryOutput{}, nil
		}
		page++
		if page == 1 {
			return &dynamodb.QueryOutput{
				Items:            []map[string]types.AttributeValue{statusItem("", "NEW", "", "2026-11-01T11:59:00.000000000Z")},
				LastEvaluatedKey: map[string]types.AttributeValue{"PK": s(pkValue)},
			}, nil
		}
		require.NotNil(t, in.ExclusiveStartKey)
		return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{
			statusItem("NEW", "IN_PROGRESS", "", "2026-11-01T12:00:00.000000000Z"),
		}}, nil
	}}

	statuses, _, err := NewDynamoStore(api, "history").ListByServiceOrder(context.Background(), soID)
	require.NoError(t, err)
	assert.Len(t, statuses, 2)
}

func TestListByServiceOrderEmptyReturnsEmptySlices(t *testing.T) {
	statuses, works, err := NewDynamoStore(&fakeAPI{}, "history").ListByServiceOrder(context.Background(), soID)
	require.NoError(t, err)
	assert.NotNil(t, statuses)
	assert.NotNil(t, works)
	assert.Empty(t, statuses)
	assert.Empty(t, works)
}

func TestListByServiceOrderQueryErrorIsWrapped(t *testing.T) {
	api := &fakeAPI{queryFn: func(*dynamodb.QueryInput) (*dynamodb.QueryOutput, error) { return nil, errBoom }}
	_, _, err := NewDynamoStore(api, "history").ListByServiceOrder(context.Background(), soID)
	assert.ErrorIs(t, err, errBoom)
}

func TestListByServiceOrderWorkQueryErrorIsWrapped(t *testing.T) {
	calls := 0
	api := &fakeAPI{queryFn: func(in *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
		calls++
		if calls == 2 {
			return nil, errBoom
		}
		return &dynamodb.QueryOutput{}, nil
	}}
	_, _, err := NewDynamoStore(api, "history").ListByServiceOrder(context.Background(), soID)
	assert.ErrorIs(t, err, errBoom)
}

func TestListByServiceOrderMalformedItemsAreErrors(t *testing.T) {
	cases := map[string]struct {
		prefix string
		item   map[string]types.AttributeValue
	}{
		"status without new_status": {"S#", map[string]types.AttributeValue{"created_at": s("2026-11-01T12:00:00.000000000Z")}},
		"status bad created_at":     {"S#", map[string]types.AttributeValue{"new_status": s("NEW"), "created_at": s("x")}},
		"work without work_id":      {"W#", map[string]types.AttributeValue{"new_status": s("NEW"), "created_at": s("2026-11-01T12:00:00.000000000Z")}},
		"work without new_status":   {"W#", map[string]types.AttributeValue{"work_id": s(workID), "created_at": s("2026-11-01T12:00:00.000000000Z")}},
		"work without created_at":   {"W#", map[string]types.AttributeValue{"work_id": s(workID), "new_status": s("NEW")}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{queryFn: routeQueries(map[string][]map[string]types.AttributeValue{tc.prefix: {tc.item}})}
			_, _, err := NewDynamoStore(api, "history").ListByServiceOrder(context.Background(), soID)
			assert.Error(t, err)
		})
	}
}

func reportItem(id, name, count, total string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"PK":            s("REPORT#WORK"),
		"SK":            s("W#" + id),
		"work_name":     s(name),
		"count":         n(count),
		"total_seconds": n(total),
	}
}

func TestListReportFiltersByWorkIDs(t *testing.T) {
	items := []map[string]types.AttributeValue{
		reportItem("w1", "Troca de óleo", "2", "7200"),
		reportItem("w2", "Revisão de freios", "1", "10800"),
	}
	api := &fakeAPI{queryFn: routeQueries(map[string][]map[string]types.AttributeValue{"W#": items})}
	st := NewDynamoStore(api, "history")

	all, err := st.ListReport(context.Background(), nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, ReportRecord{WorkID: "w1", WorkName: "Troca de óleo", Count: 2, TotalSeconds: 7200}, all[0])

	filtered, err := st.ListReport(context.Background(), []string{"w2", "missing"})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "w2", filtered[0].WorkID)

	none, err := st.ListReport(context.Background(), []string{"missing"})
	require.NoError(t, err)
	assert.Empty(t, none)
}

func TestListReportMalformedAggregatesAreErrors(t *testing.T) {
	base := func() map[string]types.AttributeValue { return reportItem("w1", "x", "1", "1") }
	cases := map[string]func(map[string]types.AttributeValue){
		"no work_name":     func(i map[string]types.AttributeValue) { delete(i, "work_name") },
		"count not number": func(i map[string]types.AttributeValue) { i["count"] = s("1") },
		"count not int":    func(i map[string]types.AttributeValue) { i["count"] = n("1.5") },
		"total missing":    func(i map[string]types.AttributeValue) { delete(i, "total_seconds") },
		"total not float":  func(i map[string]types.AttributeValue) { i["total_seconds"] = n("abc") },
		"no sort key":      func(i map[string]types.AttributeValue) { delete(i, "SK") },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			item := base()
			mutate(item)
			api := &fakeAPI{queryFn: routeQueries(map[string][]map[string]types.AttributeValue{"W#": {item}})}
			_, err := NewDynamoStore(api, "history").ListReport(context.Background(), nil)
			assert.Error(t, err)
		})
	}
}

func TestListReportQueryErrorIsWrapped(t *testing.T) {
	api := &fakeAPI{queryFn: func(*dynamodb.QueryInput) (*dynamodb.QueryOutput, error) { return nil, errors.New("down") }}
	_, err := NewDynamoStore(api, "history").ListReport(context.Background(), nil)
	assert.Error(t, err)
}
