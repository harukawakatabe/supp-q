# Supp Q server

One Go module with four independently runnable commands:

- `cmd/api` — HTTP API
- `cmd/worker` — background worker process
- `cmd/admin` — explicit administration commands
- `cmd/migrate` — embedded append-only Goose migrations

The API exposes operational endpoints plus the Phase 1 session,
authentication, password reset, and invitation administration contract. Domain,
recognition, and AI routes are not implemented yet.

## Commands

```bash
cp .env.example .env.local
go test ./...
go run ./cmd/api
go run ./cmd/worker
go run ./cmd/admin status
go run ./cmd/migrate
```

Environment variables are read from the process. The Go programs deliberately
do not load `.env` files implicitly; use the root `Makefile`, Compose, or export
the values in your shell.

## Operational endpoints

- `GET /api/v1/health/live` — process is serving HTTP; no dependency assertion.
- `GET /api/v1/health/ready` — process can authenticate to PostgreSQL and run a
  ping within the configured timeout.

Every response includes `X-Request-ID`. A caller-supplied request ID is not
trusted or reflected; the server generates its own identifier.

See `../docs/IDENTITY.md` for the local administrator bootstrap and end-to-end
identity flow. Set `SUPPQ_TEST_DATABASE_URL` to a disposable PostgreSQL database
to run the temporary-schema integration test instead of skipping it.
