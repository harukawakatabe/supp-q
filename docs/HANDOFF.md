# Coding Agent Handoff

## Current truth

This project is specification and architecture only. Do not claim that the H5 app, Go API, database, authentication, demo lifecycle, OCR, or deployment is implemented.

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

## Frozen history

The sibling `web/`, `mvp/`, and `demo/` projects are frozen. They may be inspected for:

- product flows
- information hierarchy
- visual references
- schedule rules
- FEFO allocation
- exact undo behavior
- recognition confirmation boundaries
- historical tests

Do not modify them or create runtime dependencies on them. Port rules into `uni/` with new tests.

## Next implementation order

### Phase 0: reproducible skeleton

- Install or select supported Go, Node, pnpm, and Docker versions.
- Bootstrap the official current uni-app Vue 3 TypeScript CLI template under `app/`.
- Bootstrap one Go module under `server/`.
- Add API, worker, and admin command entrypoints.
- Add local PostgreSQL, MinIO, and Mailpit.
- Add formatting, lint, unit-test, and build commands.
- Add CI that builds both client and server.

Acceptance: a fresh machine can start the empty H5, API, worker, database, object storage, and mail catcher using documented commands.

### Phase 1: contracts, database, identity

- Define OpenAPI and error envelope.
- Add migrations.
- Implement invitations, email verification, password login, email-code login, sessions, and logout.
- Implement isolated anonymous demo creation and 24-hour cleanup.
- Add minimal invitation admin page.

Acceptance: two anonymous browsers do not share data; two registered users cannot cross-read; registration invalidates and removes the originating demo.

### Phase 2: deterministic core

- Port and test dates, schedules, inventory, products, ingredients, and intake rules.
- Implement structured APIs.
- Do not introduce whole-state sync.

Acceptance: FEFO, exact undo, three-layer schedule intersection, finish-date, and expiry calculations pass unit and integration tests.

### Phase 3: capture and recognition

- Implement private upload.
- Persist recognition jobs.
- Add explicit fake adapters plus live adapters for the existing vision-provider and Kimi direction.
- Add worker retry/timeout.
- Implement confirmation UI.

Acceptance: a failed provider retains the user's input, supports retry/manual entry, and never creates an unconfirmed final plan.

### Phase 4: H5 main path

- Today, Records, Add, Cabinet, Me.
- Responsive mobile bottom navigation and desktop sidebar.
- Product-level AI explanation.
- In-app reminders.

### Phase 5: hardening and deployment

- Authorization tests.
- Browser E2E.
- Recognition evaluation set.
- Structured logs and health checks.
- Backup and restore drill.
- Caddy deployment.

## Documentation rule

After every material feature:

- move it from `Not started` to `In progress` or `Shipped`
- record missing failure states
- update the exact acceptance evidence
- keep deferred features visible

Do not write “complete” based on mocks, screenshots, or HTTP 200 alone.
