# 小补Q / Supp Q

This is the independent production-oriented repository for a supplement label
recognition, scheduling, intake, inventory, and reminder service.

## Current status

The complete product contract is confirmed in `prd/PRD.md`, and R1 implementation
planning is active. The H5 launch-beta loop is implemented and verified locally:
isolated demo,
invitation identity, Today/Records/Add/Cabinet/Me, product/schedule editing,
restock, intake/backfill/exact undo, private three-image recognition with human
confirmation, account/file deletion, and restart persistence. Production
Compose/Caddy, readiness/metrics, encrypted backup/restore scripts, recognition
evaluation, and Playwright CI are included.

The full R1 target is not implemented. Nothing is deployed to production. Live recognition has not passed an
authorized private 30–50-image evaluation, and production infrastructure has
not been selected. The accurate status is a deployable local release candidate,
not an online service. See `docs/PROJECT_STATUS.md` and
`docs/R1_GATE_CHECKLIST.md` for the exact state.

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

Use `make test`, `make check`, `make build`, `make status`,
`make test-restart-persistence`, and `make down` for the corresponding lifecycle
steps. `Makefile` supports both the standard `docker compose` plugin and the
Homebrew `docker-compose` binary.

## Project truth

- `docs/DOCUMENT_AUTHORITY.md` — active/archived document authority
- `prd/PRD.md` — confirmed complete product truth
- `docs/IMPLEMENTATION_PLAN_R1.md` — current R1 workstreams and dependency order
- `docs/R1_GATE_CHECKLIST.md` — RG0–RG9 evidence status
- `docs/EXECUTION_REGISTRY_R1.md` — work IDs, owner status, CI boundary, evidence locations
- `docs/SCHEMA_R1.md` — target physical schema and migration sequence
- `docs/DESIGN.md` — user-provided UI direction
- `docs/DESIGN_IMPLEMENTATION_R1.md` — R1 adaptation of that direction
- `docs/PROJECT_STATUS.md` — implemented versus planned
- `docs/ARCHITECTURE.md` — implemented architecture baseline
- `docs/DEVELOPMENT_VS_PRODUCTION.md` — local and production differences
- `docs/EXTERNAL_RESOURCES.md` — resources the owner must prepare
- `docs/HANDOFF.md` — next-agent entry point
- `docs/IDENTITY.md` — implemented identity flows and remaining gaps
- `docs/ERRORS.md` — stable error registry
- `contracts/openapi.yaml` — implemented HTTP contract only

Historical Launch-Beta and product-generation documents are under `archive/`
and cannot override the active set.

## Independence rule

This repository does not import, read, or execute files from the frozen legacy
`web/`, `mvp/`, `demo/`, or pre-extraction `uni/` snapshots retained in the
former parent repository.

`.github/workflows/ci.yml` is active at the repository root. It is intentionally
not duplicated into the frozen parent repository.

## Working brand

- Chinese: 小补Q
- English: Supp Q
- Status: working name, not a confirmed trademark

Formal trademark, app-store, social-handle, and domain checks remain required
before brand investment.
