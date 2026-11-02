# Go conventions

- Go 1.27, module `github.com/POS-FIAP-15SOAT-TEAM-DARK-MODE/auto-repair-shop-history-service`.
- Errors: wrap with `%w`; sentinel errors are package-level `var`. `store.ErrDuplicateMessage` is the only sentinel the
  consumer branches on.
- Times: UTC, RFC 3339. Envelope timestamps are the business time; never use the processing clock for business data.
- Context keys: gin route params are copied to the context by `routing.GinHandler` (string keys, route-scoped).
- AWS: `aws-sdk-go-v2` only. Endpoint override (`AWS_ENDPOINT_URL`) is for MiniStack only.
- Lint: `golangci-lint` as pinned in `.pre-commit-config.yaml`.
