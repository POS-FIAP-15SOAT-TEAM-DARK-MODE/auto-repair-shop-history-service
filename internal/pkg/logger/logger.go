package logger

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service/internal/pkg/env"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ServiceName is the value of the "service" field on every log line.
const ServiceName = "history-service"

type customLogger struct {
	*zap.Logger
}

type ctxKey struct{}

var (
	globalLog  *customLogger
	dsnPattern *regexp.Regexp
)

func init() {
	initGlobalLogger()
}

func initGlobalLogger() {
	globalLog = &customLogger{initLogger()}
	dsnPattern = regexp.MustCompile(`://([^:/\s]+):([^@/\s]+)@`)
}

// productionConfig is the JSON encoder every line goes through: the same field set on every service.
func productionConfig() zap.Config {
	cfg := zap.NewProductionConfig()
	cfg.EncoderConfig.TimeKey = "timestamp"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncoderConfig.MessageKey = "message"
	cfg.InitialFields = map[string]any{"service": ServiceName}
	cfg.Level = getLogLevel(env.GetString("LOG_LEVEL", "INFO"))
	return cfg
}

func initLogger() *zap.Logger {
	l, err := productionConfig().Build()
	if err != nil {
		panic(err)
	}

	return l
}

func getLogLevel(level string) zap.AtomicLevel {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return zap.NewAtomicLevelAt(zap.DebugLevel)
	case "PANIC":
		return zap.NewAtomicLevelAt(zap.PanicLevel)
	default:
		return zap.NewAtomicLevelAt(zap.InfoLevel)
	}
}

func Global() *customLogger {
	return globalLog
}

// Request attaches a per-request logger to ctx, tagging every log line with
// a request_id. If incomingID is non-empty (e.g. forwarded by an upstream
// API Gateway or a client-supplied X-Request-Id), it's reused as-is so log
// lines for the same request stay correlated across services; otherwise a
// new one is generated.
func Request(ctx context.Context, incomingID string) (context.Context, string) {
	reqId := incomingID
	if reqId == "" {
		reqId = uuid.New().String()
	}
	l := &customLogger{globalLog.With(zap.String("request_id", reqId))}
	return withContext(ctx, l), reqId
}

// WithFields returns a ctx whose logger (see Of) carries fields in addition to the ones it already has. Consumers use
// it to tag every line of a message with message_id, saga_id and service_order_id.
func WithFields(ctx context.Context, fields ...zap.Field) context.Context {
	return withContext(ctx, &customLogger{Of(ctx).With(fields...)})
}

// ReplaceGlobal swaps the global logger and returns a function that restores the previous one. Tests use it to
// capture output; production code never calls it.
func ReplaceGlobal(l *zap.Logger) (restore func()) {
	previous := globalLog
	globalLog = &customLogger{l}
	return func() { globalLog = previous }
}

func Of(ctx context.Context) *customLogger {
	if ctx == nil {
		return Global()
	}

	if l, ok := ctx.Value(ctxKey{}).(*customLogger); ok {
		return l
	}
	return Global()
}

func withContext(ctx context.Context, l *customLogger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

func (l *customLogger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, sanitizeFields(fields)...)
}

func (l *customLogger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, sanitizeFields(fields)...)
}

func (l *customLogger) Error(err error, fields ...zap.Field) {
	fields = append(fields, zap.Error(err))
	l.Logger.Error(err.Error(), sanitizeFields(fields)...)
}

// ErrorEvent logs a failed operation under an event name (<entity>.<action>). The error goes to the "error" field,
// so the message stays the event name.
func (l *customLogger) ErrorEvent(event string, err error, fields ...zap.Field) {
	fields = append(fields, zap.Error(err))
	l.Logger.Error(event, sanitizeFields(fields)...)
}

func (l *customLogger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, sanitizeFields(fields)...)
}

func (l *customLogger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, sanitizeFields(fields)...)
}

func (l *customLogger) Sync() {
	if l != nil {
		if err := l.Logger.Sync(); err != nil {
			err = fmt.Errorf("failed to sync logger: %w", err)
			l.Error(err)
		}
	}
}

func sanitizeFields(fields []zap.Field) []zap.Field {
	sanitized := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		key := strings.ToLower(strings.TrimSpace(field.Key))
		if isSensitiveKey(key) {
			sanitized = append(sanitized, zap.String(field.Key, "[REDACTED]"))
			continue
		}
		if field.Type == zapcore.StringType {
			value := redactDSNPassword(field.String)
			sanitized = append(sanitized, zap.String(field.Key, value))
			continue
		}
		sanitized = append(sanitized, field)
	}
	return sanitized
}

func isSensitiveKey(key string) bool {
	return key == "password" ||
		key == "senha" ||
		key == "pwd" ||
		key == "token" ||
		key == "authorization" ||
		key == "cpf" ||
		key == "cnpj" ||
		key == "document" ||
		key == "documento"
}

func redactDSNPassword(value string) string {
	return dsnPattern.ReplaceAllString(value, `://$1:[REDACTED]@`)
}
