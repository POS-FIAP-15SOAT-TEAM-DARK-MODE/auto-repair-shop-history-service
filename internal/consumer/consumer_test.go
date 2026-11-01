package consumer

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/messaging"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeQueue struct {
	batches   [][]messaging.Message
	deleted   []string
	deleteErr error
	recvErr   error
	onRecv    func()
}

func (q *fakeQueue) ReceiveMessage(_ context.Context, _ string, _, _ int32) ([]messaging.Message, error) {
	if q.onRecv != nil {
		q.onRecv()
	}
	if q.recvErr != nil {
		return nil, q.recvErr
	}
	if len(q.batches) == 0 {
		return nil, nil
	}
	b := q.batches[0]
	q.batches = q.batches[1:]
	return b, nil
}

func (q *fakeQueue) DeleteMessage(_ context.Context, _, receiptHandle string) error {
	if q.deleteErr != nil {
		return q.deleteErr
	}
	q.deleted = append(q.deleted, receiptHandle)
	return nil
}

type fakeStore struct {
	statusEvents []store.StatusEvent
	workEvents   []store.WorkEvent
	err          error
}

func (f *fakeStore) WriteStatusEvent(_ context.Context, e store.StatusEvent) error {
	f.statusEvents = append(f.statusEvents, e)
	return f.err
}

func (f *fakeStore) WriteWorkEvent(_ context.Context, e store.WorkEvent) error {
	f.workEvents = append(f.workEvents, e)
	return f.err
}

func (*fakeStore) ListByServiceOrder(context.Context, string) ([]store.StatusRecord, []store.WorkRecord, error) {
	return nil, nil, nil
}

func (*fakeStore) ListReport(context.Context, []string) ([]store.ReportRecord, error) {
	return nil, nil
}

// message builds an SQS message whose body is the envelope in the given fixture file.
func message(t *testing.T, fixture, receipt string) messaging.Message {
	t.Helper()
	raw, err := os.ReadFile("../../testdata/contracts/" + fixture)
	require.NoError(t, err)
	return messaging.Message{Body: string(raw), ReceiptHandle: receipt}
}

func envelopeBody(typ, payload string) string {
	return `{"message_id":"b7a1c6f0-2f64-4f6e-9a49-0d5a0b1e6c11","saga_id":"0f3c9a2e-6a1d-4a53-8a6e-5d3b1f2c7e90",` +
		`"type":"` + typ + `","occurred_at":"2026-11-01T12:00:00Z","service_order_id":"01JD8Z4R3Q8M2V7K5N9XWABCDE",` +
		`"request_id":"2c5e1f0a-9d77-4b2c-a0c1-3f8e6b7d1a22","payload":` + payload + `}`
}

func TestHandleServiceOrderStatusChangedContract(t *testing.T) {
	st := &fakeStore{}
	q := &fakeQueue{}
	c := New(q, "queue", st)

	require.NoError(t, c.Handle(context.Background(), message(t, "ServiceOrderStatusChanged.json", "r1")))

	require.Len(t, st.statusEvents, 1)
	e := st.statusEvents[0]
	assert.Equal(t, "b7a1c6f0-2f64-4f6e-9a49-0d5a0b1e6c11", e.MessageID)
	assert.Equal(t, "01JD8Z4R3Q8M2V7K5N9XWABCDE", e.ServiceOrderID)
	assert.Equal(t, "IN_DIAGNOSIS", e.PreviousStatus)
	assert.Equal(t, "AWAITING_APPROVAL", e.NewStatus)
	assert.Empty(t, e.Reason, "reason is null in the fixture")
	assert.Equal(t, 2026, e.OccurredAt.Year())
	assert.Equal(t, []string{"r1"}, q.deleted)
}

func TestHandleServiceOrderStatusChangedWithoutPrevious(t *testing.T) {
	st := &fakeStore{}
	c := New(&fakeQueue{}, "queue", st)

	require.NoError(t, c.Handle(context.Background(), message(t, "ServiceOrderStatusChanged.no-previous.json", "r1")))
	require.Len(t, st.statusEvents, 1)
	assert.Empty(t, st.statusEvents[0].PreviousStatus)
	assert.Equal(t, "NEW", st.statusEvents[0].NewStatus)
}

func TestHandleServiceOrderStatusChangedWithReason(t *testing.T) {
	st := &fakeStore{}
	c := New(&fakeQueue{}, "queue", st)
	body := envelopeBody("ServiceOrderStatusChanged", `{"previous":"AWAITING_APPROVAL","new":"REJECTED","reason":"BUDGET_REJECTED","total_amount":"221.00"}`)

	require.NoError(t, c.Handle(context.Background(), messaging.Message{Body: body, ReceiptHandle: "r"}))
	assert.Equal(t, "BUDGET_REJECTED", st.statusEvents[0].Reason)
}

func TestHandleExecutionWorkStatusChangedContract(t *testing.T) {
	st := &fakeStore{}
	q := &fakeQueue{}
	c := New(q, "queue", st)

	require.NoError(t, c.Handle(context.Background(), message(t, "ExecutionWorkStatusChanged.json", "r2")))

	require.Len(t, st.workEvents, 1)
	e := st.workEvents[0]
	assert.Equal(t, "11111111-1111-4111-8111-000000000001", e.WorkID)
	assert.Equal(t, "Troca de óleo", e.WorkName)
	assert.Equal(t, "AWAITING_START", e.PreviousStatus)
	assert.Equal(t, "IN_PROGRESS", e.NewStatus)
	assert.Equal(t, []string{"r2"}, q.deleted)
}

func TestHandleExecutionWorkStatusChangedWithoutPrevious(t *testing.T) {
	st := &fakeStore{}
	c := New(&fakeQueue{}, "queue", st)

	require.NoError(t, c.Handle(context.Background(), message(t, "ExecutionWorkStatusChanged.no-previous.json", "r")))
	require.Len(t, st.workEvents, 1)
	assert.Empty(t, st.workEvents[0].PreviousStatus)
	assert.Equal(t, "AWAITING_START", st.workEvents[0].NewStatus)
}

func TestUnhandledTypeIsDiscardedAndDeleted(t *testing.T) {
	st := &fakeStore{}
	q := &fakeQueue{}
	c := New(q, "queue", st)

	require.NoError(t, c.Handle(context.Background(), message(t, "BudgetGenerated.json", "r3")))
	assert.Empty(t, st.statusEvents)
	assert.Empty(t, st.workEvents)
	assert.Equal(t, []string{"r3"}, q.deleted)
}

func TestDuplicateMessageIsDeletedWithoutError(t *testing.T) {
	st := &fakeStore{err: store.ErrDuplicateMessage}
	q := &fakeQueue{}
	c := New(q, "queue", st)

	require.NoError(t, c.Handle(context.Background(), message(t, "ServiceOrderStatusChanged.json", "r4")))
	assert.Equal(t, []string{"r4"}, q.deleted)
}

func TestStoreErrorKeepsMessageInQueue(t *testing.T) {
	st := &fakeStore{err: errors.New("dynamo down")}
	q := &fakeQueue{}
	c := New(q, "queue", st)

	err := c.Handle(context.Background(), message(t, "ExecutionWorkStatusChanged.json", "r5"))
	require.Error(t, err)
	assert.Empty(t, q.deleted)
}

func TestInvalidEnvelopeIsAnErrorAndNotDeleted(t *testing.T) {
	q := &fakeQueue{}
	c := New(q, "queue", &fakeStore{})

	err := c.Handle(context.Background(), messaging.Message{Body: "{not json", ReceiptHandle: "r6"})
	require.Error(t, err)
	assert.Empty(t, q.deleted)
}

func TestInvalidPayloadIsAnErrorAndNotDeleted(t *testing.T) {
	cases := map[string]struct {
		typ, payload string
	}{
		"status payload not an object": {"ServiceOrderStatusChanged", `"oops"`},
		"status without new":           {"ServiceOrderStatusChanged", `{"previous":"NEW"}`},
		"work payload not an object":   {"ExecutionWorkStatusChanged", `[]`},
		"work without work_id":         {"ExecutionWorkStatusChanged", `{"work_name":"x","new":"IN_PROGRESS"}`},
		"work without work_name":       {"ExecutionWorkStatusChanged", `{"work_id":"w","new":"IN_PROGRESS"}`},
		"work without new":             {"ExecutionWorkStatusChanged", `{"work_id":"w","work_name":"x"}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			q := &fakeQueue{}
			st := &fakeStore{}
			c := New(q, "queue", st)

			err := c.Handle(context.Background(), messaging.Message{Body: envelopeBody(tc.typ, tc.payload), ReceiptHandle: "r"})
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalidPayload)
			assert.Empty(t, q.deleted)
			assert.Empty(t, st.statusEvents)
			assert.Empty(t, st.workEvents)
		})
	}
}

func TestDeleteErrorIsReturned(t *testing.T) {
	q := &fakeQueue{deleteErr: errors.New("sqs down")}
	c := New(q, "queue", &fakeStore{})

	err := c.Handle(context.Background(), message(t, "ServiceOrderStatusChanged.json", "r"))
	assert.Error(t, err)
}

func TestRunProcessesBatchesUntilCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st := &fakeStore{}
	q := &fakeQueue{
		batches: [][]messaging.Message{
			{message(t, "ServiceOrderStatusChanged.json", "a"), message(t, "BudgetGenerated.json", "b")},
		},
	}
	q.onRecv = func() {
		if len(q.batches) == 0 {
			cancel()
		}
	}
	c := New(q, "queue", st)

	require.NoError(t, c.Run(ctx))
	assert.Len(t, st.statusEvents, 1)
	assert.Equal(t, []string{"a", "b"}, q.deleted)
}

func TestRunStopsOnReceiveErrorWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	q := &fakeQueue{recvErr: errors.New("network"), onRecv: cancel}
	c := New(q, "queue", &fakeStore{})

	require.NoError(t, c.Run(ctx))
}

func TestRunKeepsLoopingAfterHandlerErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	st := &fakeStore{err: errors.New("dynamo down")}
	q := &fakeQueue{
		batches: [][]messaging.Message{{message(t, "ExecutionWorkStatusChanged.json", "a")}},
	}
	q.onRecv = func() {
		if len(q.batches) == 0 {
			cancel()
		}
	}
	require.NoError(t, New(q, "queue", st).Run(ctx))
	assert.Empty(t, q.deleted)
}

func TestRunRetriesAfterReceiveError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	calls := 0
	q := &fakeQueue{}
	q.onRecv = func() {
		calls++
		if calls == 1 {
			q.recvErr = errors.New("transient")
			return
		}
		q.recvErr = nil
		cancel()
	}
	require.NoError(t, New(q, "queue", &fakeStore{}).Run(ctx))
	assert.Equal(t, 2, calls)
}
