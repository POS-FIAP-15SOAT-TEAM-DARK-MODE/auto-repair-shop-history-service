# Testing

- Unit tests (`go test ./... --race`): fake `store.DynamoAPI` and fake `store.Store`/`messaging.Queue`. The service has
  no generated mocks.
- Coverage gate in CI: total >= 80% (the CI filter for `/mocks/` is a leftover, nothing to exclude); `internal/store` and
  `internal/consumer` >= 90%. Measured numbers are in `README.md`.
- Contract fixtures: `testdata/contracts/*.json` are the examples of `docs/fase4/contracts/messaging.md` of the
  `auto-repair-shop` repository. The consumer and envelope tests read them.
- Integration (`-tags=integration`, `internal/integration`): MiniStack in CI. The tests create their own DynamoDB table
  and SQS queue. Run them with `make test-integration` when an emulator is available.
