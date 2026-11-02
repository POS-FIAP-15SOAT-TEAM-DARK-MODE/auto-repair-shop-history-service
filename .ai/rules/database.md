# Database (DynamoDB)

Table `auto-repair-shop-<env>-history`, keys `PK`/`SK` (strings), on-demand. Model: `docs/fase4/contracts/ddl.md` of the
`auto-repair-shop` repository, section `history`.

- Every write is a `TransactWriteItems` whose first item is the dedupe marker `MSG#<message_id>` / `DEDUPE` with
  `attribute_not_exists(PK)`. A cancellation whose first reason is `ConditionalCheckFailed` is a duplicate
  (`store.ErrDuplicateMessage`); any other failure is an error and the message stays in the queue.
- Sort keys embed timestamps with the fixed-width layout in `internal/store/keys.go`. Do not change the layout: string
  order must stay chronological.
- Reads use `ConsistentRead` and follow pagination (`LastEvaluatedKey`).
- Never scan. Reads are `Query` on one partition (`begins_with` on the sort key). The only point read is the consistent
  `GetItem` of the start marker `R#<work_id>`, done in the write path before a `COMPLETED` transition.
