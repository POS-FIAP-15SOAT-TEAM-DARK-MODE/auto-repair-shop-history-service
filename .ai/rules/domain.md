# Domain

Messages handled (`docs/fase4/contracts/messaging.md` of the `auto-repair-shop` repository); everything else is
discarded with a log:

| Type | Stored as |
|---|---|
| `ServiceOrderStatusChanged` | status transition of the service order (`S#`) |
| `ExecutionWorkStatusChanged` | work transition (`W#`); marker `R#` on `IN_PROGRESS`; aggregate on `IN_PROGRESS -> COMPLETED` |

Report semantics:
- Average = `total_seconds / count / 3600`, per work, over all service orders.
- Duration = `occurred_at(COMPLETED) - occurred_at(IN_PROGRESS)`, only when the start marker exists.
- The work name comes from the event (`work_name`), the latest one wins.

History semantics (must match the JSON of the monolith's history endpoint, which clients still consume):
- `workTransitions` is attached only to the first entry with `newStatus == IN_PROGRESS`; groups are ordered by first
  appearance, statuses chronologically; the field is omitted when there are none.
