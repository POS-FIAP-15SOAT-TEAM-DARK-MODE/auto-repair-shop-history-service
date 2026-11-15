# AI Rules — Index (History & Reporting)

Single source of truth for the AI agent rules of this service. Pointer files (`CLAUDE.md`, `AGENTS.md`,
`.cursor/rules/`, `.windsurf/rules/`) only contain a routing table and the always-applied rules.

```
.ai/
  rules/          topic-scoped rules, open on demand
    architecture.md   layers, consumer -> store <- HTTP, dependency rules
    domain.md         transitions handled, report semantics
    go-conventions.md Go idioms, AWS SDK usage, no framework leakage
    database.md       DynamoDB key model and transaction rules
    api.md            endpoints, roles, error format, Swagger
    testing.md        coverage thresholds, unit vs integration
    security.md       JWT, roles, sensitive data
    observability.md  zap, log levels, redaction
    workflow.md       Conventional Commits, pre-commit hooks, dev commands
  hooks/
    check-ai-rules.sh enforces .ai/ as the only rule source (pre-commit)
```
