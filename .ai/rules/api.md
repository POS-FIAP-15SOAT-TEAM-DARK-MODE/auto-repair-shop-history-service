# API

| Method | Path | Roles |
|---|---|---|
| GET | `/v1/service-order/:id/history` | ATTENDANT, MECHANIC, ADMIN |
| GET | `/v1/reports/average-execution-time?work_id=…` | ATTENDANT, ADMIN |
| GET | `/ping`, `/metrics`, `/swagger/*any`, `/swagger.yaml` | public |

- Errors: `{"code": <status>, "errors": [...]}` for 4xx with a stack of messages; `{"code", "error"}` for 422/500.
- Empty results are `200 []`, never `null`.
- Swagger: `docs/swagger.yaml` (embedded in `docs/embed.go`). Keep it in sync with the routes.
