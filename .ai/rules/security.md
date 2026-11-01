# Security

- JWT is validated by `internal/pkg/auth` (HS256 with `JWT_SECRET`); roles come from the token only. No DB calls.
- Route roles are declared in `internal/app/routing`; a missing or wrong role is 403.
- The service stores no personal data beyond what the messages carry; never log payloads in full.
- `JWT_SECRET` is the only secret of the service (`auto-repair-shop-<env>/history`).
