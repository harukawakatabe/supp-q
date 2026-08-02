# Coding Agent Handoff

## Current truth

Phase 0, Phase 1 identity, the Phase 2 deterministic core, and the Phase 3
capture/recognition foundation are implemented and test-backed. Three private
images persist in S3-compatible storage, recognition jobs persist in
PostgreSQL, the worker records explicit provider identity, and only an edited
human-confirmation request creates the product.

Do not inflate that statement. The development stack uses `fake:development`;
the live OpenAI-compatible vision adapter has not received credentials or
passed the private recognition evaluation set. Restock UI, schedule editing,
Records, AI explanation, reminders, automated browser E2E, account deletion,
backup, and production deployment are not implemented.

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

### Completed Phase 2 core: deterministic domain

- Fixed-point dates/quantities, schedule intersection, independent anchors, and
  non-retroactive day-cycle history.
- Tenant-scoped products, ingredients, schedules, batches, intake records,
  exact allocations, and inventory events.
- FEFO, atomic insufficient-stock rejection, idempotent retry, and exact undo.
- H5 Today, Cabinet, Add Product, and isolated product seed for demo workspaces.

Acceptance passed in unit and PostgreSQL integration tests. The rebuilt Compose
stack also passed live proxied HTTP and a 390×844 browser path for Today intake,
undo, Cabinet, and Add Product. Still add schedule editing/version writes,
restock and Records surfaces, automated browser E2E, and a desktop pass before
closing the entire Phase 2 product surface.

### Phase 3 foundation: implemented locally

- Exactly three private uploads (`front`, `facts`, `expiry`), content-signature
  validation, opaque keys, authorized file reads, and demo-object cleanup.
- Persisted PostgreSQL jobs with claim leases, attempts, exponential backoff,
  provider identity, candidate/error payloads, retry, and retained failures.
- Explicit `fake:development` provider and an OpenAI-compatible live vision
  adapter. Production configuration rejects fake or incomplete live settings.
- H5 choose/capture/status/retry/manual/confirm flow. Recognition candidates
  are editable; confirmation is idempotent and is the only recognition path
  that creates a product.

Acceptance passed in Go unit and temporary-schema integration tests plus live
proxied HTTP against SeaweedFS. Provider failure preserves images, cross-tenant
set/file reads are hidden, and unconfirmed candidates never create products.

Still required before calling Phase 3 production-ready: configure and verify a
real provider, run the 30–50-image private evaluation set, implement the
non-H5 file-upload adapter, add automated browser E2E, and add an orphan-object
reconciliation job for the rare object-write/database-failure window.

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
- The worker recognition queue is real. `fake:development` proves orchestration,
  persistence, retry, and confirmation semantics only; it proves nothing about
  OCR accuracy or live provider connectivity.
- SeaweedFS is a replaceable local S3 sandbox. Production remains COS or OSS.
- The CI file becomes active only after `uni/` is a repository root.

## Documentation rule

After every material feature, update `PROJECT_STATUS.md`, this handoff, the
exact acceptance evidence, and unresolved failure states. Do not write
“complete” based on mocks, screenshots, health HTTP 200, or provider-shaped
fallback output alone.
