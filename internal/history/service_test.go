package history

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	statuses []store.StatusRecord
	works    []store.WorkRecord
	reports  []store.ReportRecord
	err      error

	gotReportIDs []string
}

func (f *fakeStore) WriteStatusEvent(context.Context, store.StatusEvent) error { return nil }
func (f *fakeStore) WriteWorkEvent(context.Context, store.WorkEvent) error     { return nil }

func (f *fakeStore) ListByServiceOrder(context.Context, string) ([]store.StatusRecord, []store.WorkRecord, error) {
	return f.statuses, f.works, f.err
}

func (f *fakeStore) ListReport(_ context.Context, ids []string) ([]store.ReportRecord, error) {
	f.gotReportIDs = ids
	return f.reports, f.err
}

var (
	t0   = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	w1   = "11111111-1111-4111-8111-000000000001"
	w2   = "11111111-1111-4111-8111-000000000002"
	name = "Troca de óleo"
)

var errBoom = errors.New("boom")

func at(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }

func TestGetHistoryRequiresID(t *testing.T) {
	_, err := NewService(&fakeStore{}).GetHistory(context.Background(), "")
	assert.ErrorIs(t, err, ErrServiceOrderIDRequired)
}

func TestGetHistoryWrapsStoreErrors(t *testing.T) {
	_, err := NewService(&fakeStore{err: errors.New("down")}).GetHistory(context.Background(), "so")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fail to fetch service order history")
}

func TestGetHistoryUnknownServiceOrderIsEmpty(t *testing.T) {
	entries, err := NewService(&fakeStore{}).GetHistory(context.Background(), "so")
	require.NoError(t, err)
	assert.NotNil(t, entries)
	assert.Empty(t, entries)
}

func TestGetHistoryAttachesWorkTransitionsOnlyToFirstInProgress(t *testing.T) {
	st := &fakeStore{
		statuses: []store.StatusRecord{
			{NewStatus: "NEW", CreatedAt: at(0)},
			{PreviousStatus: "NEW", NewStatus: "IN_PROGRESS", CreatedAt: at(1)},
			{PreviousStatus: "IN_PROGRESS", NewStatus: "IN_PROGRESS", CreatedAt: at(2)},
		},
		works: []store.WorkRecord{
			{WorkID: w2, WorkName: "b", NewStatus: "AWAITING_START", CreatedAt: at(3)},
			{WorkID: w1, WorkName: name, PreviousStatus: "AWAITING_START", NewStatus: "IN_PROGRESS", CreatedAt: at(1)},
			{WorkID: w1, WorkName: name, PreviousStatus: "IN_PROGRESS", NewStatus: "COMPLETED", CreatedAt: at(4)},
		},
	}

	entries, err := NewService(st).GetHistory(context.Background(), "so")
	require.NoError(t, err)
	require.Len(t, entries, 3)

	assert.Nil(t, entries[0].WorkTransitions, "NEW has no work transitions")
	assert.Nil(t, entries[2].WorkTransitions, "only the first IN_PROGRESS carries them")

	groups := entries[1].WorkTransitions
	require.Len(t, groups, 2)
	// Groups are ordered by first appearance in time: w1 (minute 1) before w2 (minute 3).
	assert.Equal(t, w1, groups[0].WorkID)
	require.Len(t, groups[0].Status, 2)
	assert.Equal(t, "AWAITING_START", groups[0].Status[0].PreviousStatus)
	assert.Equal(t, "COMPLETED", groups[0].Status[1].NewStatus)
	assert.Equal(t, w2, groups[1].WorkID)
}

func TestGetHistoryWithoutInProgressHasNoWorkTransitions(t *testing.T) {
	st := &fakeStore{
		statuses: []store.StatusRecord{{NewStatus: "NEW", CreatedAt: at(0)}},
		works:    []store.WorkRecord{{WorkID: w1, NewStatus: "AWAITING_START", CreatedAt: at(1)}},
	}
	entries, err := NewService(st).GetHistory(context.Background(), "so")
	require.NoError(t, err)
	assert.Nil(t, entries[0].WorkTransitions)
}

func TestGetHistoryInProgressWithoutWorksHasNoWorkTransitions(t *testing.T) {
	st := &fakeStore{statuses: []store.StatusRecord{{NewStatus: "IN_PROGRESS", CreatedAt: at(0)}}}
	entries, err := NewService(st).GetHistory(context.Background(), "so")
	require.NoError(t, err)
	assert.Nil(t, entries[0].WorkTransitions)
}

func TestGetHistoryDoesNotMutateStoreRecords(t *testing.T) {
	works := []store.WorkRecord{
		{WorkID: w2, NewStatus: "AWAITING_START", CreatedAt: at(3)},
		{WorkID: w1, NewStatus: "IN_PROGRESS", CreatedAt: at(1)},
	}
	st := &fakeStore{
		statuses: []store.StatusRecord{{NewStatus: "IN_PROGRESS", CreatedAt: at(0)}},
		works:    works,
	}
	_, err := NewService(st).GetHistory(context.Background(), "so")
	require.NoError(t, err)
	assert.Equal(t, w2, works[0].WorkID, "the input slice keeps the store order")
}

func TestHistoryJSONMatchesMonolithFormat(t *testing.T) {
	st := &fakeStore{
		statuses: []store.StatusRecord{
			{NewStatus: "NEW", CreatedAt: t0},
			{PreviousStatus: "NEW", NewStatus: "IN_PROGRESS", CreatedAt: at(1)},
		},
		works: []store.WorkRecord{
			{WorkID: w1, WorkName: name, PreviousStatus: "AWAITING_START", NewStatus: "IN_PROGRESS", CreatedAt: at(1)},
		},
	}
	entries, err := NewService(st).GetHistory(context.Background(), "so")
	require.NoError(t, err)

	raw, err := json.Marshal(MapHistory(entries))
	require.NoError(t, err)
	assert.JSONEq(t, `[
	  {"previousStatus":"","newStatus":"NEW","createdAt":"2026-11-01T12:00:00Z"},
	  {"previousStatus":"NEW","newStatus":"IN_PROGRESS","createdAt":"2026-11-01T12:01:00Z",
	   "workTransitions":[{"workId":"`+w1+`","status":[{"previousStatus":"AWAITING_START","newStatus":"IN_PROGRESS","createdAt":"2026-11-01T12:01:00Z"}]}]}
	]`, string(raw))
}

func TestEmptyHistoryJSONIsArray(t *testing.T) {
	raw, err := json.Marshal(MapHistory(nil))
	require.NoError(t, err)
	assert.Equal(t, "[]", string(raw))
}

func TestAverageExecutionTimeComputesHoursAndSortsByName(t *testing.T) {
	st := &fakeStore{reports: []store.ReportRecord{
		{WorkID: w2, WorkName: "Revisão de freios", Count: 2, TotalSeconds: 3600 * 7},
		{WorkID: w1, WorkName: name, Count: 2, TotalSeconds: 3600 * 3},
		{WorkID: "never", WorkName: "Sem conclusão", Count: 0, TotalSeconds: 0},
	}}

	report, err := NewService(st).GetAverageExecutionTime(context.Background(), []string{w1, w2})
	require.NoError(t, err)
	require.Len(t, report, 2)
	// Sorted by name ascending: "Revisão de freios" < "Troca de óleo".
	assert.Equal(t, "Revisão de freios", report[0].WorkName)
	assert.InDelta(t, 3.5, report[0].AverageExecutionTimeHours, 1e-9)
	assert.Equal(t, name, report[1].WorkName)
	assert.InDelta(t, 1.5, report[1].AverageExecutionTimeHours, 1e-9)
	assert.Equal(t, []string{w1, w2}, st.gotReportIDs)
}

func TestAverageExecutionTimeSortsTiesByWorkID(t *testing.T) {
	st := &fakeStore{reports: []store.ReportRecord{
		{WorkID: w2, WorkName: name, Count: 1, TotalSeconds: 3600},
		{WorkID: w1, WorkName: name, Count: 1, TotalSeconds: 3600},
	}}
	report, err := NewService(st).GetAverageExecutionTime(context.Background(), nil)
	require.NoError(t, err)
	assert.Equal(t, w1, report[0].WorkID)
}

func TestAverageExecutionTimeEmptyIsEmptyArray(t *testing.T) {
	report, err := NewService(&fakeStore{}).GetAverageExecutionTime(context.Background(), nil)
	require.NoError(t, err)
	raw, err := json.Marshal(MapReport(report))
	require.NoError(t, err)
	assert.Equal(t, "[]", string(raw))
}

func TestAverageExecutionTimeWrapsStoreErrors(t *testing.T) {
	_, err := NewService(&fakeStore{err: errors.New("down")}).GetAverageExecutionTime(context.Background(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "fail to fetch execution report")
}

func TestReportJSONFieldNames(t *testing.T) {
	raw, err := json.Marshal(MapReport([]ReportEntry{{WorkID: w1, WorkName: name, AverageExecutionTimeHours: 1.5}}))
	require.NoError(t, err)
	assert.JSONEq(t, `[{"workId":"`+w1+`","workName":"Troca de óleo","averageExecutionTimeHours":1.5}]`, string(raw))
}
