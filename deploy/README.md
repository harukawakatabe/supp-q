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

## Production

`Caddyfile.production.example` is a non-secret template, not a deployable final
configuration. Production still requires selected domains, TLS-ready DNS,
secrets, external backup placement, monitoring, and live provider verification.
