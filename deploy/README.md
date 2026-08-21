# Deployment assets

## Local development stack

`compose.dev.yml` starts six long-running services plus a one-shot migration
gate:

- H5 + Caddy: <http://127.0.0.1:3000>
- Go API (through Caddy): <http://127.0.0.1:3000/api/v1/health/live>
- PostgreSQL: `127.0.0.1:5432`
- SeaweedFS S3 API: <http://127.0.0.1:8333>
- SeaweedFS admin UI: <http://127.0.0.1:23646>
- Mailpit UI: <http://127.0.0.1:8025>
- Goose migration container: exits successfully before API and worker start

From `uni/`:

```bash
make dev
```

The Makefile prefers `docker compose` and falls back to the Homebrew
`docker-compose` binary. Direct equivalents remain valid when needed.

The local credentials committed in the development Compose file are disposable
and must never be reused outside local development.

Local email codes are visible only in Mailpit. Create the first administrator
with the command documented in `../docs/IDENTITY.md`; no public endpoint can
promote an account.

## Why SeaweedFS, not MinIO

MinIO's [community repository](https://github.com/minio/minio) was archived on
2026-04-25 and its current instructions describe source-only community
distribution. Phase 0
therefore uses actively maintained SeaweedFS only as a local S3-compatible
sandbox. Production remains private Tencent COS or Alibaba OSS behind a storage
adapter; SeaweedFS is not a production commitment.

## Production release candidate

`compose.production.yml` runs a one-shot migration followed by independent API,
worker, H5, and Caddy edge services. Start from the non-secret examples:

- `api.env.production.example`
- `worker.env.production.example`
- `backup.env.example`

Store filled files outside the checkout (for example under root-readable
`/etc/suppq/`) and never commit them. Validate interpolation before mutation:

```bash
SUPPQ_HOSTNAME=suppq.example.com \
SUPPQ_ACME_EMAIL=ops@example.com \
SUPPQ_API_ENV_FILE=/etc/suppq/api.env \
SUPPQ_WORKER_ENV_FILE=/etc/suppq/worker.env \
docker compose -f deploy/compose.production.yml config
```

Then run the server-image configuration preflights with the corresponding
environment. `Caddyfile.production` obtains HTTPS automatically, denies public
`/metrics`, and applies CSP/HSTS/COOP and other security headers.

Encrypted database + object backups:

```bash
./scripts/backup.sh /etc/suppq/backup.env
./scripts/restore-drill.sh /etc/suppq/backup.env /off-host/suppq-TIMESTAMP.tar.age
```

Restore targets must be separate, empty, and contain `drill` in both the
database URL and object-bucket name. The scripts require PostgreSQL client
tools, MinIO `mc`, and `age` on the operations host.

These assets are deployable but not accepted production evidence until domains,
TLS-ready DNS, secrets, external backup/monitoring, a real restore drill, and a
private live-provider evaluation are complete.
