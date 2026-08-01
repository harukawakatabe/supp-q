# 小补Q / Supp Q

`uni/` is the independent production-oriented project for a supplement label
recognition, scheduling, intake, inventory, and reminder service.

## Current status

Phase 0 is implemented and verified locally: a uni-app H5 shell, Go API,
separate worker and admin commands, PostgreSQL, private local S3 sandbox,
Mailpit, Caddy, pinned containers, tests, and build commands run together.

No production product feature is shipped yet. Authentication, invitations,
isolated demo identities, product records, schedules, inventory, intake,
recognition, AI, reminders, migrations, and production deployment remain
unimplemented. The Today content is visibly labelled preview data.

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
