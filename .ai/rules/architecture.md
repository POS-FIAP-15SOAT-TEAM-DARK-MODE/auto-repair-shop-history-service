# Architecture

Two entry points share one store, with no direct dependency between them:

- **Consumer** (`internal/consumer`): reads `history-queue`, decodes the envelope (`internal/pkg/messaging`), maps the
  payload to a `store.StatusEvent` / `store.WorkEvent` and calls `Store.Write*`. Deletes the message only after success,
  duplicate, or discard.
- **HTTP API** (`internal/history` + `internal/app/routing`): `Controller` -> `Service` -> `Store.List*`.

Rules:
- `internal/store` is the only package that knows DynamoDB (keys, attribute names, transactions).
- `internal/history` and `internal/consumer` depend on the `store.Store` interface, never on the SDK.
- Routing and bootstrap are wiring only (`internal/app`). No business logic in controllers.
- The service publishes nothing: there is no outbox, relay or SNS here.
