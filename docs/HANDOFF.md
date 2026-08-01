# Coding Agent Handoff

## Current truth

Phase 0 is locally operational and evidence-backed. The H5 shell, Go process
layout, PostgreSQL readiness, worker heartbeat, local S3 sandbox, Mailpit, and
Caddy path work together.

Do not inflate that statement. Authentication, invitations, demo identities,
domain records, recognition, AI, reminders, migrations, browser E2E, backup,
and production deployment are not implemented. Static Today cards are preview
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

## Start and verify

From `uni/`:

```bash
make dev
make status
curl http://127.0.0.1:3000/api/v1/health/ready
make test
make check
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

### Phase 1: contracts, database, identity

1. Define the stable error registry and identity/invitation/demo OpenAPI paths.
2. Choose and add a Go migration tool; create append-only initial migrations.
3. Implement the shared `invitations` model for generic codes and email-bound
   invitations.
4. Implement email verification, password login, email-code login, sessions,
   logout, and binding multiple auth identities.
5. Implement isolated seeded `demo_ephemeral` users and 24-hour inactivity
   cleanup.
6. On registration/login, create an empty real workspace, invalidate the demo
   session, and enqueue full demo deletion. Do not migrate or retain demo data.
7. Add minimal invitation administration without exposing it publicly.

Acceptance: two anonymous browsers never share writes; two registered users
cannot cross-read; login invalidates and deletes the originating demo; both
invitation modes use the same persistence model.

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
  working queue.
- SeaweedFS is a replaceable local S3 sandbox. Production remains COS or OSS.
- The CI file becomes active only after `uni/` is a repository root.

## Documentation rule

After every material feature, update `PROJECT_STATUS.md`, this handoff, the
exact acceptance evidence, and unresolved failure states. Do not write
“complete” based on mocks, screenshots, health HTTP 200, or provider-shaped
fallback output alone.
