package consumer

import (
	"context"
	"errors"
	"testing"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/logger"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/messaging"
	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

const statusPayload = `{"previous":"IN_DIAGNOSIS","new":"AWAITING_APPROVAL","reason":null}`

func captureLogs(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	core, logs := observer.New(zap.DebugLevel)
	restore := logger.ReplaceGlobal(zap.New(core))
	t.Cleanup(restore)
	return logs
}

// entry returns the first log line with the given event name.
func entry(t *testing.T, logs *observer.ObservedLogs, event string) observer.LoggedEntry {
	t.Helper()
	for _, e := range logs.All() {
		if e.Message == event {
			return e
		}
	}
	require.FailNowf(t, "event not logged", "event %q not found", event)
	return observer.LoggedEntry{}
}

func TestHandle_AppliedCarriesCorrelationFields(t *testing.T) {
	logs := captureLogs(t)
	c := New(&fakeQueue{}, "queue", &fakeStore{})

	require.NoError(t, c.Handle(context.Background(),
		messaging.Message{Body: envelopeBody("ServiceOrderStatusChanged", statusPayload), ReceiptHandle: "r"}))

	fields := entry(t, logs, "message.applied").ContextMap()
	assert.Equal(t, "b7a1c6f0-2f64-4f6e-9a49-0d5a0b1e6c11", fields["message_id"])
	assert.Equal(t, "0f3c9a2e-6a1d-4a53-8a6e-5d3b1f2c7e90", fields["saga_id"])
	assert.Equal(t, "01JD8Z4R3Q8M2V7K5N9XWABCDE", fields["service_order_id"])
	assert.Equal(t, "ServiceOrderStatusChanged", fields["type"])
}

func TestHandle_DuplicateIsLoggedAndDeleted(t *testing.T) {
	logs := captureLogs(t)
	q := &fakeQueue{}
	c := New(q, "queue", &fakeStore{err: store.ErrDuplicateMessage})

	require.NoError(t, c.Handle(context.Background(),
		messaging.Message{Body: envelopeBody("ServiceOrderStatusChanged", statusPayload), ReceiptHandle: "r"}))

	assert.Equal(t, "b7a1c6f0-2f64-4f6e-9a49-0d5a0b1e6c11", entry(t, logs, "message.duplicate").ContextMap()["message_id"])
	assert.Equal(t, []string{"r"}, q.deleted)
}

func TestHandle_UnhandledTypeIsDiscardedWithWarning(t *testing.T) {
	logs := captureLogs(t)
	c := New(&fakeQueue{}, "queue", &fakeStore{})

	require.NoError(t, c.Handle(context.Background(),
		messaging.Message{Body: envelopeBody("BudgetGenerated", `{}`), ReceiptHandle: "r"}))

	e := entry(t, logs, "message.discarded")
	assert.Equal(t, zap.WarnLevel, e.Level)
	assert.Equal(t, "unhandled_type", e.ContextMap()["reason"])
	assert.Equal(t, "BudgetGenerated", e.ContextMap()["type"])
}

func TestHandle_InvalidEnvelopeIsLoggedAndKept(t *testing.T) {
	logs := captureLogs(t)
	q := &fakeQueue{}
	c := New(q, "queue", &fakeStore{})

	require.Error(t, c.Handle(context.Background(), messaging.Message{Body: `{"not":"an envelope"}`, ReceiptHandle: "r"}))

	e := entry(t, logs, "message.invalid")
	assert.Equal(t, zap.ErrorLevel, e.Level)
	assert.NotEmpty(t, e.ContextMap()["error"])
	assert.Empty(t, q.deleted, "an invalid message stays in the queue")
}

func TestHandle_ApplyFailureIsLoggedAndKept(t *testing.T) {
	logs := captureLogs(t)
	q := &fakeQueue{}
	c := New(q, "queue", &fakeStore{err: errors.New("dynamo down")})

	require.Error(t, c.Handle(context.Background(),
		messaging.Message{Body: envelopeBody("ServiceOrderStatusChanged", statusPayload), ReceiptHandle: "r"}))

	fields := entry(t, logs, "message.failed").ContextMap()
	assert.Equal(t, "b7a1c6f0-2f64-4f6e-9a49-0d5a0b1e6c11", fields["message_id"])
	assert.Contains(t, fields["error"], "dynamo down")
	assert.Empty(t, q.deleted)
}

func TestHandle_DeleteFailureIsLogged(t *testing.T) {
	logs := captureLogs(t)
	q := &fakeQueue{deleteErr: errors.New("sqs unavailable")}
	c := New(q, "queue", &fakeStore{})

	require.Error(t, c.Handle(context.Background(),
		messaging.Message{Body: envelopeBody("ServiceOrderStatusChanged", statusPayload), ReceiptHandle: "r"}))

	assert.Contains(t, entry(t, logs, "message.delete_failed").ContextMap()["error"], "sqs unavailable")
}

// failOnceQueue fails the first receive and cancels the run on the second one.
type failOnceQueue struct {
	fakeQueue
	calls  int
	cancel context.CancelFunc
}

func (q *failOnceQueue) ReceiveMessage(_ context.Context, _ string, _, _ int32) ([]messaging.Message, error) {
	q.calls++
	if q.calls == 1 {
		return nil, errors.New("network")
	}
	q.cancel()
	return nil, nil
}

func TestRun_ReceiveFailureIsLogged(t *testing.T) {
	logs := captureLogs(t)
	ctx, cancel := context.WithCancel(context.Background())
	c := New(&failOnceQueue{cancel: cancel}, "queue", &fakeStore{})

	require.NoError(t, c.Run(ctx))

	assert.Equal(t, zap.ErrorLevel, entry(t, logs, "queue.receive_failed").Level)
}
