# Auto Repair Shop — History & Reporting Service

Read model of the Auto Repair Shop. It keeps the history of service orders and works, and the average execution time
of each work. It consumes events from the saga (`so-events`, `billing-events`, `execution-events`) and publishes nothing.

- Port `8085`, namespace `auto-repair-shop`, Deployment/Service `history-service`.
- Storage: DynamoDB only (no PostgreSQL, no migrations).
- Contracts shared with the other services: `docs/fase4/contracts/` in the `auto-repair-shop` repository
  (`resources.md`, `messaging.md`, `ddl.md` section `history`).

## Endpoints

| Method | Path | Roles |
|---|---|---|
| GET | `/v1/service-order/:id/history` | ATTENDANT, MECHANIC, ADMIN |
| GET | `/v1/reports/average-execution-time?work_id=…` | ATTENDANT, ADMIN |

Swagger: `/swagger/index.html` (spec in `docs/swagger.yaml`). Operational: `/ping`, `/metrics`.

`GET /history` returns the same JSON as the monolith (`previousStatus`, `newStatus`, `createdAt`, and `workTransitions`
on the first `IN_PROGRESS` entry, grouped by work). An unknown service order returns `[]`.

## Messages

| Type | Action |
|---|---|
| `ServiceOrderStatusChanged` | stores the transition (`S#`) |
| `ExecutionWorkStatusChanged` | stores the transition (`W#`); on `IN_PROGRESS` writes the start marker (`R#`); on `IN_PROGRESS -> COMPLETED`, adds the duration to the report aggregate |
| anything else | discarded with a log and removed from the queue |

Each message is applied in one `TransactWriteItems` that starts with the dedupe marker `MSG#<message_id>`. A redelivery
writes nothing and adds nothing to the aggregate.

## Data model (table `auto-repair-shop-<env>-history`, keys `PK`/`SK`)

| Item | `PK` | `SK` |
|---|---|---|
| Service-order transition | `SO#<id>` | `S#<occurred_at>#<message_id>` |
| Work transition | `SO#<id>` | `W#<work_id>#<occurred_at>#<message_id>` |
| Start marker | `SO#<id>` | `R#<work_id>` |
| Report aggregate per work | `REPORT#WORK` | `W#<work_id>` (`count`, `total_seconds`, `work_name`) |
| Dedupe | `MSG#<message_id>` | `DEDUPE` |

`occurred_at` in keys uses a fixed-width UTC layout with nanoseconds, so string order is chronological.

## Limitations

- **Out-of-order messages.** SQS Standard does not keep order. A transition is stored when it arrives. If
  `ExecutionWorkStatusChanged` `COMPLETED` arrives before the `IN_PROGRESS` that created the start marker, the event is
  stored but the duration is not added to the report (logged as a warning).
- **Same timestamp.** Two transitions of the same work with identical `occurred_at` are ordered by `message_id`. The
  saga does not produce them, but the order between them is then arbitrary.
- The work name in the report is the latest `work_name` received for that work.

## Configuration

| Variable | Purpose | Default |
|---|---|---|
| `PORT` | HTTP port | `8085` |
| `JWT_SECRET` | token validation (secret `auto-repair-shop-<env>/history`). Unset, the code falls back to the placeholder `your_jwt_secret`: set it in every deployed environment | `your_jwt_secret` |
| `AWS_REGION` | region | `us-east-1` |
| `AWS_ENDPOINT_URL` | MiniStack endpoint (local and CI only) | empty |
| `DYNAMODB_TABLE` | read-model table (required) | — |
| `HISTORY_QUEUE_URL` | `history-queue`; empty disables the consumer | empty |
| `LOG_LEVEL` | `DEBUG`, `INFO`, `PANIC` | `INFO` |

## Local development

```bash
cp .env.example .env
make up          # docker compose: app + MiniStack; aws-init creates the queue, subscriptions and table
curl -fsS localhost:8085/ping
make down
```

`local/init-aws.sh` is idempotent and creates only this service's resources (prefix `auto-repair-shop-local-`).

## Tests

```bash
make test              # unit tests, race detector
make coverage          # coverage summary
make test-integration  # needs AWS_ENDPOINT_URL pointing to MiniStack
```

Measured with `make coverage` on 2026-11-01 (the tree has no generated mocks, so nothing is excluded): total **90.6%**.

| Package | Coverage |
|---|---|
| `internal/store` | 100.0% |
| `internal/consumer` | 100.0% |
| `internal/history` | 100.0% |
| `internal/app/middleware`, `internal/app/routing`, `internal/pkg/app` | 100.0% |
| `internal/pkg/web` | 96.3% |
| `internal/pkg/logger` | 93.4% |
| `internal/pkg/messaging` | 46.2% |
| `cmd/service`, `internal/app`, `internal/app/bootstrap` | 0% (wiring and error variables, no unit tests) |

CI runs `pre-commit`, `build` (`go build`, `go vet` with and without `-tags=integration`), `unit-tests` (race detector),
`integration-tests` (MiniStack), `coverage` (gate: 80% total, 90% for `store` and `consumer`) and `sonarqube`.

## Decisions

See `docs/adr/`: branching strategy (0001) and the DynamoDB read model with transactional dedupe (0002).
