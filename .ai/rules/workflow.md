# Workflow

## Conventional Commits
`<type>(<scope>): <subject>`; types `feat fix refactor test chore docs perf style revert`; subject <= 72 chars,
lowercase, imperative, no trailing period. Scopes used here: `bootstrap`, `messaging`, `store`, `consumer`, `history`,
`infra`, `ci`, `docs`.

## Pre-commit hooks
Configured in `.pre-commit-config.yaml`: whitespace and YAML checks, `golangci-lint`, `go fmt`, `go build`,
`go vet`, unit tests, and `check-ai-rules` (rule files live only under `.ai/`).

## Development commands
```bash
make run              # start the service (needs .env, see .env.example)
make test             # unit tests with the race detector
make test-integration # integration tests (needs AWS_ENDPOINT_URL=MiniStack)
make coverage         # coverage profile and summary
make up / make down   # docker compose: app + MiniStack + aws-init
```
