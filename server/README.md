# Supp Q server

One Go module with five independently runnable commands:

- `cmd/api` — HTTP API
- `cmd/worker` — background worker process
- `cmd/admin` — explicit administration commands
- `cmd/migrate` — embedded append-only Goose migrations
- `cmd/eval` — offline recognition quality-gate report

The API exposes operations, identity/account deletion, product/schedule/batch,
intake/history, and private upload/recognition/confirmation routes. The worker
processes recognition, expired-demo/account object cleanup, orphan
reconciliation, and a readiness heartbeat. Recognition uses an explicit Fake
development provider or configured live adapters. Reminder times/summaries are
domain/client state; Web Push and product AI explanation are deferred.

## Commands

```bash
cp .env.example .env.local
go test ./...
go run ./cmd/api
go run ./cmd/worker
go run ./cmd/admin status
go run ./cmd/admin validate-config --worker
go run ./cmd/migrate
go run ./cmd/eval -input /absolute/path/to/results.json
```

Environment variables are read from the process. The Go programs deliberately
do not load `.env` files implicitly; use the root `Makefile`, Compose, or export
the values in your shell.

## Operational endpoints

- `GET /api/v1/health/live` — process is serving HTTP; no dependency assertion.
- `GET /api/v1/health/ready` — database, private object bucket, worker heartbeat,
  and recognition queue are available.
- `GET /metrics` — internal Prometheus text for HTTP, queue, and worker state;
  production Caddy denies this path externally.

Every response includes `X-Request-ID`. A caller-supplied request ID is not
trusted or reflected; the server generates its own identifier.

See `../docs/IDENTITY.md` for the local administrator bootstrap and end-to-end
identity flow. Set `SUPPQ_TEST_DATABASE_URL` to a disposable PostgreSQL database
to run the temporary-schema identity, catalog, and recognition integration
tests instead of skipping them. Recognition also needs the S3-compatible
object variables listed in `.env.example`; production refuses a fake provider
or incomplete live-provider configuration.
