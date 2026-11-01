# AGENTS.md

> **Single source of truth: `.ai/`**
> Only this file is auto-loaded by Codex / GitHub Copilot. All linked files must be opened on demand.
> Never duplicate rule content here.

## Always-Applied Rules

### 1. Security
- Never log or expose JWT tokens, JWT secrets or customer documents (CPF/CNPJ) in any response, log entry or error message.
- Validate input at the handler layer; never trust raw strings inside the consumer or the store.

### 2. Git Conventions
All commits follow Conventional Commits: `<type>(<scope>): <description>`.
Types: `feat fix refactor test chore docs perf style revert` — scopes in `.ai/rules/workflow.md`.

### 3. Layer Contract (verify after every edit)
- Controllers: no business logic. Service: no DynamoDB types. Store: no HTTP.
- Consumer: no reads of the read model; it only applies messages through the `Store` interface.

---

## Task Routing

| Task type | Files to read |
|---|---|
| New feature or change of the read model | `.ai/rules/architecture.md` · `.ai/rules/domain.md` · `.ai/rules/database.md` |
| API endpoint | `.ai/rules/api.md` · `.ai/rules/security.md` |
| Auth or security | `.ai/rules/security.md` |
| Tests | `.ai/rules/testing.md` |
| Logging or observability | `.ai/rules/observability.md` |
| Git workflow | `.ai/rules/workflow.md` |

All rule files: `.ai/rules/` · Index: `.ai/README.md`
