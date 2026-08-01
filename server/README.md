# Supp Q server

One Go module with three independently runnable commands:

- `cmd/api` — HTTP API
- `cmd/worker` — background worker process
- `cmd/admin` — explicit administration commands

The Phase 0 API exposes only operational endpoints. Domain, authentication,
invitation, demo, recognition, and AI routes are not implemented yet.

## Commands

```bash
cp .env.example .env.local
go test ./...
go run ./cmd/api
go run ./cmd/worker
go run ./cmd/admin status
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
