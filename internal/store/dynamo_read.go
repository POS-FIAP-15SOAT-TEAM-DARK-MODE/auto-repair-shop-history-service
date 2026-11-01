package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ListByServiceOrder queries the status items (S#) and the work items (W#) of one service order.
// Both lists come back in sort-key order, which is chronological because the keys embed occurred_at.
func (s *DynamoStore) ListByServiceOrder(ctx context.Context, serviceOrderID string) ([]StatusRecord, []WorkRecord, error) {
	pk := soPK(serviceOrderID)

	statusItems, err := s.queryAll(ctx, pk, statusSKPrefix)
	if err != nil {
		return nil, nil, err
	}
	workItems, err := s.queryAll(ctx, pk, workSKPrefix)
	if err != nil {
		return nil, nil, err
	}

	statuses := make([]StatusRecord, 0, len(statusItems))
	for _, item := range statusItems {
		rec, err := decodeStatus(item)
		if err != nil {
			return nil, nil, err
		}
		statuses = append(statuses, rec)
	}

	works := make([]WorkRecord, 0, len(workItems))
	for _, item := range workItems {
		rec, err := decodeWork(item)
		if err != nil {
			return nil, nil, err
		}
		works = append(works, rec)
	}
	return statuses, works, nil
}

// ListReport returns the report aggregates, filtered by work id when workIDs is not empty.
func (s *DynamoStore) ListReport(ctx context.Context, workIDs []string) ([]ReportRecord, error) {
	items, err := s.queryAll(ctx, reportPK, reportWorkSKPrefix)
	if err != nil {
		return nil, err
	}

	wanted := make(map[string]struct{}, len(workIDs))
	for _, id := range workIDs {
		wanted[id] = struct{}{}
	}

	records := make([]ReportRecord, 0, len(items))
	for _, item := range items {
		rec, err := decodeReport(item)
		if err != nil {
			return nil, err
		}
		if _, ok := wanted[rec.WorkID]; len(workIDs) > 0 && !ok {
			continue
		}
		records = append(records, rec)
	}
	return records, nil
}

// queryAll reads every page of a partition whose sort key starts with prefix.
func (s *DynamoStore) queryAll(ctx context.Context, pk, prefix string) ([]map[string]types.AttributeValue, error) {
	var (
		items    []map[string]types.AttributeValue
		startKey map[string]types.AttributeValue
	)
	for {
		out, err := s.api.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(s.table),
			ConsistentRead:         aws.Bool(true),
			ExclusiveStartKey:      startKey,
			KeyConditionExpression: aws.String("#pk = :pk AND begins_with(#sk, :prefix)"),
			ExpressionAttributeNames: map[string]string{
				"#pk": attrPK,
				"#sk": attrSK,
			},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pk":     sAttr(pk),
				":prefix": sAttr(prefix),
			},
		})
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", pk, err)
		}
		items = append(items, out.Items...)
		if len(out.LastEvaluatedKey) == 0 {
			return items, nil
		}
		startKey = out.LastEvaluatedKey
	}
}

func decodeStatus(item map[string]types.AttributeValue) (StatusRecord, error) {
	newStatus, err := stringAttr(item, "new_status")
	if err != nil {
		return StatusRecord{}, err
	}
	createdAt, err := timeAttr(item, "created_at")
	if err != nil {
		return StatusRecord{}, err
	}
	return StatusRecord{
		PreviousStatus: optionalString(item, "previous_status"),
		NewStatus:      newStatus,
		Reason:         optionalString(item, "reason"),
		CreatedAt:      createdAt,
	}, nil
}

func decodeWork(item map[string]types.AttributeValue) (WorkRecord, error) {
	workID, err := stringAttr(item, "work_id")
	if err != nil {
		return WorkRecord{}, err
	}
	newStatus, err := stringAttr(item, "new_status")
	if err != nil {
		return WorkRecord{}, err
	}
	createdAt, err := timeAttr(item, "created_at")
	if err != nil {
		return WorkRecord{}, err
	}
	return WorkRecord{
		WorkID:         workID,
		WorkName:       optionalString(item, "work_name"),
		PreviousStatus: optionalString(item, "previous_status"),
		NewStatus:      newStatus,
		CreatedAt:      createdAt,
	}, nil
}

func decodeReport(item map[string]types.AttributeValue) (ReportRecord, error) {
	workName, err := stringAttr(item, "work_name")
	if err != nil {
		return ReportRecord{}, err
	}
	rawCount, err := numberAttr(item, "count")
	if err != nil {
		return ReportRecord{}, err
	}
	count, err := strconv.ParseInt(rawCount, 10, 64)
	if err != nil {
		return ReportRecord{}, fmt.Errorf("parse count: %w", err)
	}
	rawTotal, err := numberAttr(item, "total_seconds")
	if err != nil {
		return ReportRecord{}, err
	}
	total, err := strconv.ParseFloat(rawTotal, 64)
	if err != nil {
		return ReportRecord{}, fmt.Errorf("parse total_seconds: %w", err)
	}
	// The aggregate sort key is "W#<work_id>"; the id is taken from it so it does not depend on extra attributes.
	sk, err := stringAttr(item, attrSK)
	if err != nil {
		return ReportRecord{}, err
	}
	return ReportRecord{
		WorkID:       strings.TrimPrefix(sk, reportWorkSKPrefix),
		WorkName:     workName,
		Count:        count,
		TotalSeconds: total,
	}, nil
}

func timeAttr(item map[string]types.AttributeValue, name string) (time.Time, error) {
	raw, err := stringAttr(item, name)
	if err != nil {
		return time.Time{}, err
	}
	t, err := parseTime(raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse %s: %w", name, err)
	}
	return t, nil
}
