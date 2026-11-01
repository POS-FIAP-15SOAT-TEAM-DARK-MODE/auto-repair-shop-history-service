# Observability

- Logs: zap via `internal/pkg/logger`. `LOG_LEVEL` = DEBUG | INFO | PANIC.
- Consumer logs: duplicate (INFO), unhandled type (WARN), start marker missing on completion (WARN), errors (ERROR with
  `component=sqs-consumer`). Include `message_id` and `type`, never the payload.
- Metrics: `http_request_duration_seconds` at `/metrics` (route label is the matched template).
