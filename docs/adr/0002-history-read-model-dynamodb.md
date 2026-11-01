# ADR 0002 — History read model on DynamoDB, written by transactions with a dedupe marker

- Status: Accepted
- Date: 2026-11-01
- Tags: history, dynamodb, messaging, idempotency

## Context

The monolith stored service-order and work transitions in PostgreSQL and computed the execution report with SQL joins.
The History & Reporting service is a read model fed by `ServiceOrderStatusChanged` and `ExecutionWorkStatusChanged`
messages. Those messages are delivered at least once and in no guaranteed order (SQS Standard). The service has no
Postgres database.

## Decision

- Store everything in one DynamoDB table (`PK`/`SK`, on-demand), with the keys defined in `contracts/ddl.md`.
- Apply every message in one `TransactWriteItems`: a dedupe item `MSG#<message_id>` / `DEDUPE` with
  `attribute_not_exists(PK)`, plus the transition item and, when applicable, the start marker or the report aggregate.
  A cancelled transaction whose first reason is `ConditionalCheckFailed` is a duplicate; any other failure is an error and
  the message stays in the queue.
- Compute the execution time from `occurred_at` of the envelopes, never from the processing clock. The start marker
  `R#<work_id>` is written on `IN_PROGRESS`; the duration is added to `REPORT#WORK` / `W#<work_id>` on
  `IN_PROGRESS -> COMPLETED`, only when the marker exists.
- Sort keys embed timestamps in a fixed-width UTC layout with nanoseconds, so that string order is chronological.

## Consequences

- Redelivery is idempotent: the same `message_id` writes nothing and adds nothing to the aggregate.
- Reads are two `Query` calls per service order and one per report; the report aggregate is a single item per work.
- A transition received before its predecessor is stored as it arrives. If `COMPLETED` arrives before `IN_PROGRESS`
  the duration is not computed (no marker). This is a known limitation (see README).
- The `TransactWriteItems` dedupe was validated against MiniStack by `internal/integration`, which runs in CI.
