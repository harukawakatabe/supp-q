# 小补Q / Supp Q

`uni/` is the independent production-oriented project for a supplement label
recognition, scheduling, intake, inventory, and reminder service.

## Current status

Phase 0, Phase 1 identity, and the Phase 2 deterministic core are implemented
and verified locally. The H5 now reads tenant-scoped products, three-layer
schedules, batch inventory, Today progress, FEFO intake allocation, and exact
undo from PostgreSQL. New demo sessions receive isolated sample products.

Nothing is deployed to production. Recognition, AI, reminders, records and
restock UI, storage cleanup, production email, automated browser E2E, backups,
and production deployment remain unimplemented.

## Start locally

Prerequisites: Go 1.26.5, Node 24.14.0, pnpm 11.9.0, and Docker with Compose.

```bash
make dev
```

Then open:

- H5: <http://127.0.0.1:3000>
- API readiness: <http://127.0.0.1:3000/api/v1/health/ready>
- Mailpit: <http://127.0.0.1:8025>
- local S3: <http://127.0.0.1:8333>

Use `make test`, `make check`, `make build`, `make status`, and `make down` for
the corresponding lifecycle steps. `Makefile` supports both the standard
`docker compose` plugin and the Homebrew `docker-compose` binary.

## Project truth

- `PRD.md` — product truth
- `docs/PROJECT_STATUS.md` — implemented versus planned
- `docs/ARCHITECTURE.md` — target architecture and current skeleton
- `docs/DEVELOPMENT_VS_PRODUCTION.md` — local and production differences
- `docs/EXTERNAL_RESOURCES.md` — resources the owner must prepare
- `docs/HANDOFF.md` — next-agent entry point
- `docs/IDENTITY.md` — implemented identity flows and remaining gaps
- `docs/ERRORS.md` — stable error registry
- `contracts/openapi.yaml` — implemented HTTP contract only

## Independence rule

This directory must remain movable as a standalone repository. It does not
import, read, or execute files from sibling `web/`, `mvp/`, or `demo/`.

`uni/.github/workflows/ci.yml` becomes an active GitHub workflow after `uni/`
is moved to its own repository root. It is intentionally not duplicated into
the frozen parent repository.

## Working brand

- Chinese: 小补Q
- English: Supp Q
- Status: working name, not a confirmed trademark

Formal trademark, app-store, social-handle, and domain checks remain required
before brand investment.
