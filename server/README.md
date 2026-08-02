# Supp Q server

One Go module with four independently runnable commands:

- `cmd/api` — HTTP API
- `cmd/worker` — background worker process
- `cmd/admin` — explicit administration commands
- `cmd/migrate` — embedded append-only Goose migrations

The API exposes operational endpoints, Phase 1 identity, the Phase 2 product
and intake domain, and Phase 3 private upload/recognition/confirmation routes.
The worker processes durable recognition jobs through an explicit fake
development provider or a configured OpenAI-compatible vision adapter. AI
explanation and reminder routes are not implemented.

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
to run the temporary-schema identity, catalog, and recognition integration
tests instead of skipping them. Recognition also needs the S3-compatible
object variables listed in `.env.example`; production refuses a fake provider
or incomplete live-provider configuration.
