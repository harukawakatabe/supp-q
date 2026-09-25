# Project Status

Last updated: 2026-09-25

## Direct conclusion

The complete product contract is now confirmed (`CONTRACT_READY`) and R1
implementation planning is active. The code remains a locally verified H5
Launch-Beta release candidate (`IMPLEMENTATION_PARTIAL`), not a live production
service (`PRODUCTION_NOT_DEPLOYED`). The code contains the launch-beta product loop, deployment
composition, health/metrics, cleanup, backup/restore tooling, and automated
acceptance. Production promotion is blocked by owner-provided infrastructure
and a real-label provider evaluation, while complete R1 promotion is also
blocked by the target schema/API/migration/UI/observability work in
`IMPLEMENTATION_PLAN_R1.md`.

The active product contract is `../prd/PRD.md`. The old Launch-Beta PRD and
acceptance document are archived and no longer define the maximum scope.

Confidence:

- **High** — local deterministic domain, identity, private-file orchestration,
  H5 main path, restart persistence, production configuration guards.
- **Medium** — production deployment assets and encrypted backup/restore
  scripts are syntax/config validated but have not run against selected cloud
  resources.
- **Unknown** — production recognition accuracy, email delivery, backup
  restore, DNS/TLS, and alert delivery until real services are selected and
  accepted.

## Current execution phase

- R1 workstreams and dependencies: `IMPLEMENTATION_PLAN_R1.md`.
- Execution/owners: `EXECUTION_REGISTRY_R1.md`; actual human owner names and
  branch-protection enforcement remain open.
- Physical schema: `SCHEMA_R1.md`; the platform spine, E2
  Product/Profile/Ingredient, E3 ProductPlan/ScheduleVersion, and E4
  Capture/Evidence migrations are locally verified, while E5-E6 and target-read
  cutover remain open.
- Release evidence: `R1_GATE_CHECKLIST.md`; RG0–RG4 are partial where noted and
  no production Gate is accepted for target R1.
- UI direction: user-provided `DESIGN.md`, adapted through
  `DESIGN_IMPLEMENTATION_R1.md`.
- No complete R1 vertical slice has been claimed; the S1 platform spine, E2
  Product/Profile/Ingredient, E3 ProductPlan/ScheduleVersion, and E4
  Capture/Evidence expand/backfill sub-slices have `VERIFIED_LOCAL` evidence.

## Shipped to production

Nothing. No production domain, server, database, object bucket, email provider,
or accepted recognition provider has been supplied.

## Implemented and verified locally

### H5 product loop

- Isolated anonymous demo plus invitation registration, email-code/password
  login, reset, logout, and role-gated invitation administration.
- Today, Records, Add, Cabinet, product detail/edit, and Me navigation on mobile
  and desktop H5.
- Manual product creation; three-image upload; durable recognition polling,
  retry/manual fallback, saved OCR evidence, editable candidate, and explicit
  human-confirmation boundary.
- Product basic/status editing, schedule versioning, weekly/day/long-cycle
  rules, reminder times, restock thresholds, expiry settings, and new batches.
- Scheduled/ad-hoc/backfilled intake records, notes, FEFO inventory deduction,
  exact repeat-safe undo, 30-day record list, projected finish/latest start,
  and low-stock/expiry risk.
- In-app reminder summary. H5 Web Push remains deferred.
- Account deletion UI and API: exact-email confirmation immediately invalidates
  all sessions; the worker deletes private objects first and then cascades
  registered account data, retrying failed cleanup jobs.

### Data, recognition, and tenancy

- PostgreSQL migrations through `202609200003`. The R1 platform spine adds
  workspace timezone versions, ClientAction, DomainChange, consumer receipts,
  projection revisions, and unsupported-product quarantine. E2 adds immutable
  ProductProfile/IngredientProfile versions, media/deletion support tables,
  durable batched catch-up state, Product pointers, and batch formula binding.
  E3 adds ProductPlan, immutable ScheduleVersion, DoseSlot, PlanStateInterval,
  ScheduledOccurrence identity/snapshots, and a resumable plan catch-up state.
  E4 adds CaptureDraft, fixed CaptureSlots, immutable SlotVersions,
  version-bound target recognition jobs and attempts, FileLinks, immutable OCR
  evidence/candidates, editable ConfirmationDrafts, and resumable catch-up state.
- Fresh and synthetic Launch-Beta snapshot upgrades, resumable E2 catch-up,
  N-1 drift snapshots, compatibility paths, parent cleanup, pre-target-write
  rollback, and post-target-write rollback refusal pass for E2 implementation
  commit `e667a08`.
- Fresh/synthetic E3 upgrade, one-row resume, idempotent rerun, N-1 drift,
  quarantine, tenant/immutability/interval constraints, parent cascade,
  deterministic occurrence identity, and guarded Down pass for implementation
  commit `fcc9dda`. This evidence remains E3-specific and does not prove E5-E6
  or target-read cutover.
- Fresh/synthetic E4 upgrade, idempotent and N-1 catch-up, partial-role mapping,
  target-owned image replacement without a legacy Job, attempt-reclaim and
  replaced-image late-result isolation, immutable evidence/candidates, active
  file protection, parent cleanup, SQL inventory/reconciliation execution, and
  guarded Down pass for implementation commit `1cc7f0c`. This does not mean
  independent per-slot APIs/H5 states, E5-E6, or target-read cutover exist.
- Product create/update and recognition confirmation dual-write E2 profiles and
  E3 plan facts transactionally while legacy reads remain authoritative.
  Metadata-only/repeated plan updates do not create target schedule versions;
  pause/resume is independent of depletion, intake, restock, and undo. Existing/new
  batches bind the applicable IngredientProfile. OTC/prescription rows are
  quarantined and hidden; new ordinary creation is rejected.
- Legacy recognition creation/worker/retry paths now dual-write E4 capture
  identities and append attempt-bound evidence/candidates. Confirmation creates
  Product/profile/plan/opening inventory, capture terminal state, source/media
  links, and DomainChange in one transaction. Legacy recognition reads remain
  authoritative; the legacy endpoint has no fabricated ClientAction.
- The shadow evaluator enforces plan-start/effective boundaries, weekly/day/
  long-cycle intersection, deterministic per-slot IDs, IANA timezones, and the
  fixed DST gap/fold policy. It is not connected to Today reads and does not
  materialize an unbounded future horizon.
- Six-decimal quantities,
  transactionally consistent FEFO allocation, idempotency, and tenant scope on
  private resources.
- Day-cycle history remains non-retroactive when the current plan changes.
- Private S3-compatible JPEG/PNG/WebP storage, opaque keys, authorized no-store
  reads, expired-demo cleanup, deleted-account cleanup, and orphan-object
  reconciliation with a grace period.
- Durable PostgreSQL recognition jobs with leases, bounded retries, retained
  failures, provider/model/timing provenance, and OCR-before-structuring
  persistence.
- Explicit `fake:development`, direct OpenAI-compatible vision, and
  evidence-pipeline adapters. Production rejects Fake and incomplete selected
  provider configuration.
- Recognition-evaluation CLI and documented 30–50-image quality gate covering
  field accuracy, correction, unrecognized/false-confidence/provider-failure
  rates, and p95 latency.

### Operations and deployment

- Production Compose for migration gate, API, worker, static H5, and Caddy edge;
  backend-only network, read-only containers, dropped capabilities, automatic
  TLS, CSP/HSTS/COOP, security headers, and external metrics denial.
- Readiness checks PostgreSQL, private object bucket, worker heartbeat, and
  recognition queue. Local Compose fixes SeaweedFS development volumes at 64
  MB and requires at least 14 free volume slots before starting API and worker;
  this reserves both seven-volume growth batches needed by mini mode's default
  and upload collections. `/metrics` exposes request, queue, and worker gauges
  for an internal collector.
- Per-IP API/auth rate limits with explicit trusted-proxy configuration.
- Production configuration rejects insecure origin, database/object transport,
  weak token pepper, non-TLS SMTP, untrusted proxy, Fake recognition, and
  incomplete provider stages. `suppq-admin validate-config [--worker]` provides
  a no-network preflight.
- SMTP client uses STARTTLS with TLS 1.2 minimum in production and bounded
  deadlines.
- Encrypted `age` backup and guarded restore-drill scripts cover PostgreSQL plus
  private objects, verify SHA-256 manifests, and refuse restore targets that do
  not contain `drill`.
- CI builds client/server/evaluation commands, runs unit and PostgreSQL
  integration tests, starts the complete stack, bootstraps a local admin, and
  runs Playwright with retained failure artifacts.

## Acceptance evidence on 2026-08-21

- `make check`: pass.
- `make test`: pass.
- `make build`: pass for H5 and API/worker/admin/migrate/eval commands.
- PostgreSQL integration suites: pass for identity, catalog, and recognition,
  including tenant isolation, account cleanup, file cleanup, orphan
  reconciliation, schedule history, FEFO, retry, and confirmation.
- Rebuilt seven-service local Compose: migration applied; API, worker,
  PostgreSQL, object storage, Mailpit, and H5 started successfully.
- `/api/v1/health/ready`: `database`, `storage`, and `worker` all `ready`;
  local provider remains visibly `fake:development`.
- `/metrics`: request counters, recognition queue gauges, and live worker
  heartbeat returned from the API and remain blocked by the public production
  Caddy route definition.
- `make test-restart-persistence`: pass after real PostgreSQL/API/worker
  restarts; the same session and quantity remained, then the test intake was
  undone and the exact original quantity restored.
- Playwright: isolated demos/main tabs, manual create→restock→backfill→undo,
  three-image Fake candidate→human confirmation, invitation registration→H5
  account deletion, and mobile/desktop overflow coverage pass against the live
  local stack. Browser console errors are rejected.
- Production Compose configuration, shell syntax for backup/restore/restart
  scripts, and current Caddy configuration validate locally.

The Playwright suite uses generated one-pixel images only. No private user
image was sent to a provider in this work.

## Standalone CI hardening evidence on 2026-09-19

- Hosted run `35452193408` reproduced the fresh-run failure: client and server
  passed, while mobile Fake recognition timed out because SeaweedFS reported
  `0 node candidates` after its default collection consumed all seven
  auto-sized volume slots.
- An isolated fresh local project with 64 MB development volumes reported
  `Max=239, Free=239`; after the 60-second automatic default growth it retained
  `Free=232` with volumes `1-7` allocated.
- Production-style readiness, administrator bootstrap, and PostgreSQL
  integration suites passed against that isolated stack.
- Playwright passed `8`, skipped the two intentional cross-project duplicates,
  and failed `0`; the mobile three-image upload, Fake candidate, and human
  confirmation path completed after the default seven-volume allocation.
- Evidence uses generated one-pixel images and `fake:development`; it proves
  local/CI orchestration, not recognition accuracy or production storage.
- The repaired main baseline then passed hosted run `35453181201` at commit
  `04538ce`; its retained integration artifact digest is
  `sha256:25db16dbc8743ad7126889e7faef39cd3447e37b6c286e428d82a1b9af0519c7`.

## Remaining production gates

These are not code-completion claims:

1. Select domain/DNS, server/region, TLS-ready network, production PostgreSQL,
   private COS/OSS, SMTP provider, monitoring destination, and off-host backup
   destination.
2. Record the selected visual/LLM provider's retention terms and obtain explicit
   authorization for a 30–50-image private evaluation set.
3. Run `make recognition-eval INPUT=...`; do not promote a provider unless every
   threshold passes.
4. Deploy with real secret files, run `suppq-admin validate-config` for API and
   `--worker` for worker, apply migrations, and verify readiness/metrics without
   exposing `/metrics` publicly.
5. Run an encrypted backup and a separate empty-target restore drill. Retain the
   manifest, table/object counts, and timestamp.
6. Pass the full browser main path against production-like email, storage, and
   live recognition; verify account image deletion and alert delivery.
7. Complete privacy/terms copy naming the actual third-party processors and
   finish working-name/domain/trademark clearance before public marketing.

Until those seven gates have evidence, the accurate state is **deployable
release candidate**, not **online production**.

## Complete target not yet implemented

- Remaining R1 occurrence materialization/service cutover, independent per-slot
  capture APIs/H5 replace-skip-manual flow, enriched intake/inventory,
  risk/reminder schema and projections, target-read cutover, target UI and
  release gates.
- R2 cost ledger, ingredient understanding/calendar, exports, and manual notes.
- R3 controlled supplement AI and purpose-bound minimum health context.
- R4 external notifications, WeChat login/upload and mini-program UI.
- Public registration, payments, collaboration, commerce, and prescription/OTC
  management remain outside the current confirmed delivery scope.

## Known debt

- The official uni-app template still pins `vue-i18n` v9 for compatibility.
- The uni-app/Vite pipeline emits Dart Sass legacy API deprecation warnings.
- Automatic email-bound invitation delivery is not implemented; the admin UI
  returns a one-time secret for deliberate sharing. Login/reset codes do use
  SMTP.
- Production backup tooling depends on `pg_dump`/`pg_restore`, MinIO `mc`, and
  `age`; those tools and real destinations are external operational resources.
- `.github/workflows/ci.yml` is active in the independent `supp-q` repository.
  The GitHub API reported the repository as public on 2026-09-20. The owner
  selected direct development-branch delivery without PR or `main` merge; the
  current workflow therefore provides no hosted branch run. Branch protection
  and required-check enforcement remain open RG1 work.
