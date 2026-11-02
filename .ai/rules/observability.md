# Observability

- Logs: zap via `internal/pkg/logger`, JSON on stdout. `LOG_LEVEL` = DEBUG | INFO | PANIC.
- Every line carries `service` (`history-service`). A request line carries `request_id` (the incoming `X-Request-Id`
  or a new UUID, echoed back in the response header). A message line carries `message_id`, `saga_id` and
  `service_order_id` from the envelope, attached to the context logger by the consumer (`logger.WithFields`).
- Event names follow `<entity>.<action>`: `request.started`, `request.completed`, `message.applied`,
  `message.duplicate`, `message.discarded` (WARN), `message.invalid`, `message.failed`, `message.delete_failed`,
  `queue.receive_failed`, `work.start_marker_missing` (WARN), `history.query_failed`, `report.query_failed`,
  `server.*`, `middleware.mounted`. Failures use `logger.ErrorEvent(event, err, fields...)`: the event is the message
  and the error goes to the `error` field.
- Never log: JWT, `Authorization`, JWT secret, CPF/CNPJ, message payloads. `sanitizeFields` redacts the sensitive keys
  (`authorization`, `token`, `password`, `cpf`, `cnpj`, `document`) and DSN passwords as a second line of defence.
- Metrics: `http_request_duration_seconds` at `/metrics` (route label is the matched template).
- Not implemented here, on purpose: `outbox_pending` (this service has no outbox), `saga_*` and
  `saga_step_duration_seconds` (saga metrics belong to the service-order service), `dlq_messages` (would need the DLQ
  URL in the config and a `GetQueueAttributes` call in the queue port; not done).
