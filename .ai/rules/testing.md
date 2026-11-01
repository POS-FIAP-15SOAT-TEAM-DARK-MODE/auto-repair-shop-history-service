# Testing

- Unit tests (`go test ./... --race`): fake `store.DynamoAPI` and fake `store.Store`/`messaging.Queue`.
- Coverage gate in CI: total >= 80% without generated mocks; `internal/store` and `internal/consumer` >= 90%.
- Contract fixtures: `testdata/contracts/*.json` are the examples of `contracts/messaging.md`. The consumer tests read them.
- Integration (`-tags=integration`, `internal/integration`): MiniStack in CI. The table and the queue are created by the
  tests. Run them with `make test-integration` when an emulator is available.
