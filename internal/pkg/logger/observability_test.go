package logger

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func captureGlobal(t *testing.T) *observer.ObservedLogs {
	t.Helper()
	core, logs := observer.New(zap.DebugLevel)
	restore := ReplaceGlobal(zap.New(core))
	t.Cleanup(restore)
	return logs
}

func TestProductionConfig_ServiceField(t *testing.T) {
	assert.Equal(t, "history-service", ServiceName)
	assert.Equal(t, ServiceName, productionConfig().InitialFields["service"])
	assert.Equal(t, "timestamp", productionConfig().EncoderConfig.TimeKey)
}

func TestRequest_TagsLinesWithRequestID(t *testing.T) {
	logs := captureGlobal(t)

	ctx, reqID := Request(context.Background(), "req-1")
	Of(ctx).Info("request.started")

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]
	assert.Equal(t, "request.started", entry.Message)
	assert.Equal(t, "req-1", reqID)
	assert.Equal(t, "req-1", entry.ContextMap()["request_id"])
}

func TestWithFields_AddsToRequestLogger(t *testing.T) {
	logs := captureGlobal(t)

	ctx, _ := Request(context.Background(), "req-2")
	ctx = WithFields(ctx, zap.String("message_id", "m-1"), zap.String("saga_id", "s-1"))
	Of(ctx).Info("message.applied")

	fields := logs.All()[0].ContextMap()
	assert.Equal(t, "req-2", fields["request_id"])
	assert.Equal(t, "m-1", fields["message_id"])
	assert.Equal(t, "s-1", fields["saga_id"])
}

func TestWithFields_WithoutRequestLogger(t *testing.T) {
	logs := captureGlobal(t)

	ctx := WithFields(context.Background(), zap.String("message_id", "m-2"))
	Of(ctx).Info("message.applied")

	assert.Equal(t, "m-2", logs.All()[0].ContextMap()["message_id"])
}

func TestErrorEvent_UsesEventAsMessage(t *testing.T) {
	logs := captureGlobal(t)

	Global().ErrorEvent("message.failed", errors.New("boom"), zap.String("type", "X"))

	entry := logs.All()[0]
	assert.Equal(t, zap.ErrorLevel, entry.Level)
	assert.Equal(t, "message.failed", entry.Message)
	assert.Equal(t, "boom", entry.ContextMap()["error"])
	assert.Equal(t, "X", entry.ContextMap()["type"])
}

func TestErrorEvent_RedactsSensitiveFields(t *testing.T) {
	logs := captureGlobal(t)

	Global().ErrorEvent("auth.failed", errors.New("bad"),
		zap.String("authorization", "Bearer abc.def.ghi"),
		zap.String("cpf", "12345678901"),
	)

	fields := logs.All()[0].ContextMap()
	assert.Equal(t, "[REDACTED]", fields["authorization"])
	assert.Equal(t, "[REDACTED]", fields["cpf"])
}

func TestReplaceGlobal_Restores(t *testing.T) {
	before := Global()
	core, _ := observer.New(zap.InfoLevel)
	restore := ReplaceGlobal(zap.New(core))
	assert.NotSame(t, before, Global())

	restore()
	assert.Same(t, before, Global())
}
