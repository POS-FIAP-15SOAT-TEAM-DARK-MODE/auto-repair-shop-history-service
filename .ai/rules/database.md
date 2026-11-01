# Database (DynamoDB)

Table `auto-repair-shop-<env>-history`, keys `PK`/`SK` (strings), on-demand. Model: `contracts/ddl.md`, section `history`.

- Every write is a `TransactWriteItems` whose first item is the dedupe marker `MSG#<message_id>` / `DEDUPE` with
  `attribute_not_exists(PK)`.
- Sort keys embed timestamps with the fixed-width layout in `internal/store/keys.go`. Do not change the layout: string
  order must stay chronological.
- Reads use `ConsistentRead` and follow pagination (`LastEvaluatedKey`).
- Never scan. Every read is a `Query` on a partition.
