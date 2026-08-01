# Coding Agent Handoff

## Current truth

Phase 0 and the Phase 1 identity core are locally operational and
evidence-backed. The H5 shell, Go process layout, migrations, PostgreSQL,
isolated demo sessions, invitations, email codes, password login/reset, worker
cleanup, local S3 sandbox, Mailpit, and Caddy path work together.

Do not inflate that statement. Domain records, seeded demo products,
recognition, AI, reminders, storage cleanup, automated browser E2E, backup, and
production deployment are not implemented. Static Today cards are preview
content, not user data or functioning schedule records.

## Read first

1. `../AGENTS.md`
2. `../PRD.md`
3. `DECISIONS.md`
4. `ARCHITECTURE.md`
5. `PROJECT_STATUS.md`
6. `DEVELOPMENT_VS_PRODUCTION.md`
7. `EXTERNAL_RESOURCES.md`
8. `SECURITY.md`
9. `ACCEPTANCE.md`
10. `../contracts/openapi.yaml`
11. `IDENTITY.md`
12. `ERRORS.md`

## Start and verify

From `uni/`:

```bash
make dev
make status
curl http://127.0.0.1:3000/api/v1/health/ready
make test
make check
```

For the first local administrator:

```bash
docker-compose -f deploy/compose.dev.yml exec -T api \
  /app/suppq-admin bootstrap-admin admin@suppq.local
```

Use `make down` to stop containers without deleting the local PostgreSQL and
object-storage volumes. Do not delete volumes unless the user explicitly asks
to destroy local data.

## Frozen history

The sibling `web/`, `mvp/`, and `demo/` projects are frozen. They may be
inspected for product evidence but must not be modified or imported. Port rules
into `uni/` with new tests and provenance notes.

## Completed Phase 0

- Supported Go, Node, pnpm, and container versions selected.
- Current official uni-app Vue 3 TypeScript dependency baseline pinned.
- H5 and future WeChat targets compile.
- API, worker, and admin commands compile and have initial tests.
- PostgreSQL, SeaweedFS S3 sandbox, Mailpit, Caddy, API, worker, and H5 start
  through Compose.
- Operational contract, health checks, request IDs, CORS, structured logs,
  environment examples, Make targets, Dockerfiles, and standalone CI added.
- Mobile and desktop Today shell checked in a real browser.

## Next implementation order

### Completed Phase 1 core: contracts, database, identity

- Stable errors and OpenAPI identity paths.
- Embedded Goose migrations.
- Shared generic and email-bound invitations with atomic claim.
- Email-code and password identities, reset, session rotation, and logout.
- Isolated demo identities and 24-hour database cleanup.
- Empty real workspace on login; originating demo queued for deletion.
- Role-gated H5 invitation administration and CLI admin bootstrap.

Still required before calling all Phase 1 production-ready: automatic
invitation-email delivery, IP/proxy abuse controls, automated browser E2E, and
file cleanup after file persistence exists. Sample demo products move into
Phase 2 because their domain schema does not exist yet.

### Phase 2: deterministic core

- Port and test dates, schedules, inventory, products, ingredients, and intake.
- Do not introduce whole-state sync.

Acceptance: FEFO, exact undo, three-layer schedule intersection, finish-date,
and expiry calculations pass unit and integration tests.

### Phase 3: capture and recognition

- Private upload and persisted jobs.
- Explicit fake adapters plus live adapters for the existing vision-provider
  and Kimi direction.
- Retry, timeout, failure retention, manual entry, and confirmation UI.

Acceptance: provider failure preserves user input and never creates an
unconfirmed final plan.

### Phase 4: H5 main path

- Today, Records, Add, Cabinet, Me.
- Product-level AI explanation.
- In-app reminders.

### Phase 5: hardening and deployment

- Full authorization tests, browser E2E, recognition evaluation set, backup and
  restore drill, monitoring, and Caddy production release.

## Known traps

- `vue-i18n` v9 is official-template compatibility debt, not an endorsed new
  dependency choice.
- Sass legacy API warnings are real debt but do not currently fail builds.
- Liveness never proves database health; use readiness for PostgreSQL.
- Worker `jobs: not_implemented` is intentional and must not be presented as a
  working domain queue. Demo cleanup is the only active worker job.
- SeaweedFS is a replaceable local S3 sandbox. Production remains COS or OSS.
- The CI file becomes active only after `uni/` is a repository root.

## Documentation rule

After every material feature, update `PROJECT_STATUS.md`, this handoff, the
exact acceptance evidence, and unresolved failure states. Do not write
“complete” based on mocks, screenshots, health HTTP 200, or provider-shaped
fallback output alone.
