# API

| Method | Path | Roles |
|---|---|---|
| GET | `/v1/service-order/:id/history` | ATTENDANT, MECHANIC, ADMIN |
| GET | `/v1/reports/average-execution-time?work_id=…` | ATTENDANT, ADMIN |
| GET | `/ping`, `/metrics`, `/swagger/*any`, `/swagger.yaml` | public |

- Error bodies (built by `internal/pkg/web` and the auth middleware):
  - 400 `{"code", "errors": [...]}`: only the texts of the validation sentinels. No route answers 400 today.
  - 401 `{"code", "error"}`: `Unauthorized` or `Token expired`. 403 `{"code", "error"}`: `Forbidden`.
  - 422 `{"code", "error"}`: the domain text (service order id required) or `invalid data` for an infrastructure violation.
  - 500 `{"code", "error"}`: `internal server error`. Causes (driver or AWS messages) stay in the server log.
- Empty results are `200 []`, never `null`.
- Swagger: `docs/swagger.yaml` (embedded in `docs/embed.go`). It documents every route in the table above.
