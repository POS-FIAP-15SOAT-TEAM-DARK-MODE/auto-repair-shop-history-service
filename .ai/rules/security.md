# Security

- JWT is validated by `internal/pkg/auth` (HS256 with `JWT_SECRET`); roles come from the token only. No DB calls.
- Route roles are declared in `internal/app/routing`. A missing or invalid token is 401; a valid token without one of the
  route roles is 403.
- The service stores no personal data beyond what the messages carry; never log payloads in full.
- `JWT_SECRET` is the only secret of the service (`auto-repair-shop-<env>/history`). Known gap: if it is unset the code
  falls back to the placeholder `your_jwt_secret` (`internal/pkg/auth/jwt.go`). Every deployed environment must set it.
