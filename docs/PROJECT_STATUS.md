# Project Status

Last updated: 2026-08-02

## Shipped to production

Nothing in `uni/` is deployed or serving real users yet.

## Implemented and verified locally

- Official current uni-app Vue 3 + TypeScript CLI dependency baseline, limited
  to H5 and future WeChat mini-program targets.
- Notion-like warm-neutral Today shell with explicit Phase 0 preview labeling.
- Responsive mobile bottom navigation and desktop sidebar shell.
- Client API health request with online/offline state.
- H5 and `mp-weixin` builds, TypeScript check, and six client scaffold tests.
- Go 1.26.5 module with independent API, worker, and admin commands.
- Server-generated request IDs, JSON logs, strict configured-origin CORS,
  liveness, and PostgreSQL-backed readiness.
- Eight Go tests across configuration, CORS, liveness, and readiness behavior.
- OpenAPI 3.1 operational contract for implemented health endpoints.
- Pinned local stack: PostgreSQL 17.10, SeaweedFS 4.29, Mailpit 1.30.0,
  Caddy 2.11.4, API, and worker.
- Local S3 credentials and pre-created private `suppq-uploads` bucket.
- Multi-stage non-root Go container and build-tested H5 container.
- Make targets, tool versions, environment examples, and standalone CI file.
- Goose 3.27.1 embedded, append-only identity migration and a migration gate
  that completes before API and worker startup.
- Isolated PostgreSQL-backed `demo_ephemeral` users, workspaces, HttpOnly
  sessions, 24-hour rolling inactivity expiry, and worker cleanup.
- Shared generic-code/email-bound invitation lifecycle with expiry, maximum
  use, single-use email binding, revocation, acceptance rows, and atomic claim.
- Email verification-code login through SMTP/Mailpit, Argon2id password login,
  password reset with all-session invalidation, logout, and two email auth
  identities bound to the same user.
- Minimal H5 authentication and invitation-administration pages. Invitation
  secrets are returned only once; the initial admin is bootstrapped by CLI.
- Stable Phase 1 error registry and OpenAPI 0.2 identity contract.
- PostgreSQL product, ingredient, schedule, day-cycle history, inventory batch,
  intake, allocation, and inventory-event schema with user/workspace scope on
  every private lookup and mutation.
- Six-decimal fixed-point quantities, three-layer schedule intersection,
  independent cycle anchors, historical day-cycle evaluation, projected finish,
  latest-start, and expiry-risk calculations.
- FEFO allocation with `NULL` expiry last, all-or-nothing stock checks,
  idempotent intake creation, exact allocation-backed undo, and auditable stock
  events in one transaction.
- Deterministic demo seed containing D3, magnesium, and fish oil. Existing empty
  demo workspaces are seeded lazily; registered workspaces remain empty.
- Real H5 Today, Cabinet, and Add Product pages backed by the Phase 2 API. The
  previous static preview cards were removed.
- OpenAPI 0.3 domain contract and stable Phase 2 error codes.
- Append-only Phase 3 schema for private file metadata, three-role recognition
  sets, durable jobs, attempts, leases, result provenance, and idempotent
  product linkage.
- S3-compatible private object adapter, validated JPEG/PNG/WebP uploads (10 MB
  each), opaque object keys, tenant-scoped no-store file delivery, and worker
  cleanup before expired-demo database deletion.
- PostgreSQL recognition worker with `SKIP LOCKED` claiming, two-minute leases,
  bounded attempts, exponential retry, retained failures, and explicit
  provider/model identity.
- Explicit `fake:development` candidate provider and an OpenAI-compatible live
  vision adapter. Production configuration rejects fake or incomplete live
  provider settings; no live provider has been configured or accepted.
- H5 capture path for front, facts, and expiry images; persisted job polling,
  per-job retry, manual fallback, visible Fake banner, editable prefill, and an
  idempotent human-confirmation boundary before product creation.
- OpenAPI 0.4 recognition contract and stable Phase 3 error codes.

## Acceptance evidence on 2026-08-01 and 2026-08-02

- `pnpm install --frozen-lockfile`: pass with four explicitly allowed build
  dependencies.
- `pnpm type-check`: pass.
- `pnpm test`: 4/4 pass.
- `pnpm build:h5`: pass.
- `pnpm build:mp-weixin`: pass.
- `go test ./...`: pass with writable isolated build cache.
- `go vet ./...`: pass.
- `go build ./cmd/api ./cmd/worker ./cmd/admin`: pass.
- `docker-compose ... config`: pass.
- All six runtime containers start; PostgreSQL, API, and Mailpit report healthy.
- Direct liveness, direct readiness, and Caddy-proxied readiness return real
  responses; readiness includes `database: ready`.
- Worker logs a successful database heartbeat and explicitly reports jobs as
  `not_implemented`.
- SeaweedFS logs confirm the `suppq-uploads` bucket was created; anonymous S3
  root access returns HTTP 403.
- Mailpit reports v1.30.0 through its live API.
- Browser check at 390x844 confirms API connected, zero horizontal overflow,
  fixed bottom navigation, and no clipped cards. Desktop 1280px breakpoint has
  fixed sidebar navigation and zero horizontal overflow.
- Identity integration test creates its own PostgreSQL schema and passes demo
  isolation, generic/email-bound invitations, concurrent one-use claim,
  email-code plus password login, password reset, session invalidation, and
  demo cleanup.
- Live Mailpit delivery received six-digit codes; a real generic invitation was
  claimed, use count advanced atomically, and the same user re-entered with a
  password in a separate cookie session.
- Live email-bound request returned 400 for the wrong email and 202 for the
  bound email.
- Real H5 browser at 390x844 completed demo → password login → registered empty
  workspace → logout → new isolated demo. Admin login exposed the invitation
  page with both created invitations and no horizontal overflow.
- Phase 2 pure Go tests pass schedule intersection, rest-day finish projection,
  independent anchors, historical day-cycle rules, fixed-point arithmetic,
  FEFO, atomic insufficient-stock rejection, and exact restoration.
- Phase 2 PostgreSQL integration test creates a fresh schema and passes scoped
  product access, FEFO split allocation, idempotent retry, no partial mutation,
  exact and repeat-safe undo, cross-tenant read/undo hiding, and Today projection.
- `pnpm type-check`, 5/5 client tests, and `pnpm build:h5` pass for the real
  Today/Cabinet/Add client.
- Rebuilt Compose applied migration `202608020001`; live Caddy-proxied HTTP
  created an isolated three-product demo, deducted D3 from 28 to 27, replayed
  the same idempotency key without another deduction, and restored the exact
  batch to 28 on undo.
- Real H5 at 390×844 loaded 3 calculated Today items, changed 0/3 → 1/3 on
  intake and back to 0/3 on undo, displayed the real Cabinet batches and risk,
  opened the Add Product form, had no horizontal overflow, and logged no
  browser console warnings or errors.
- Phase 3 provider unit tests verify explicit Fake provenance, non-final Fake
  status, response normalization, and refusal to accept an expiry date without
  matching visible evidence.
- Phase 3 temporary-schema integration test persists three images and jobs,
  proves no unconfirmed product exists, hides sets/files/products across
  tenants, runs all jobs, confirms idempotently, retains provider failures, and
  requeues a failed job without deleting its image.
- Rebuilt local stack applied migrations `202608020002` and `202608020003`.
  Live proxied multipart HTTP persisted three 26,247-byte JPEG files, the
  worker completed three jobs
  as `partial` with provider `fake:development`, and an authorized file read
  returned the original JPEG while a request without a session returned 401.
- Live confirmation created product `78e367af-eee5-413a-942f-bf77e29f90c5` only
  after an edited confirmation request; repeating confirmation returned the
  same product, and the recognition set records that product ID.
- Rebuilt H5 browser verification exposed separate three-image and manual-entry
  paths, explicit private/Fake-provider copy, all three required image roles,
  and no browser console warnings or errors.

## In progress

- Filling Phase 2 surface gaps: restock UI, schedule editing/version creation,
  and a Records page.
- Adding automated browser E2E and a repeatable restart-persistence check; the
  current H5 acceptance is manual browser evidence.
- Selecting/configuring the real vision provider and building a private
  30–50-image recognition evaluation set. Adapter code alone is not provider
  acceptance.
- Adding the uni-app non-H5 upload adapter and object-orphan reconciliation.

## Not started

- Records, restock, product-detail/edit, and account-deletion H5 interactions.
- Application reminders.
- Automated browser E2E suites (domain authorization has PostgreSQL integration coverage).
- Production deployment, backup, restore, monitoring, and alerting.
- Automatic delivery of email-bound invitation links through the selected
  production email provider.
- Reverse-proxy/IP abuse limits and automated browser E2E for identity paths.
- Demo sample intake history; Phase 2 currently seeds products and batches, not
  historical intake records.

## Deferred after V1

- H5 Web Push.
- WeChat login and mini-program subscription messages.
- Data export.
- Complete AI conversation and note organization.
- Advanced cost analytics.
- Public registration.
- Payments and plans.

## Known debt and environment notes

- The current official uni-app template constrains `vue-i18n` to unsupported
  v9. It remains pinned for template compatibility and must be evaluated before
  product localization work.
- The uni-app/Vite pipeline calls Dart Sass's deprecated legacy JS API. Builds
  pass, but this must be resolved before Dart Sass 2 adoption.
- The CI file lives under `uni/.github/` and activates only when `uni/` becomes
  its own repository root, preserving the frozen parent boundary.
- The first Colima 0.10.3 VM on this machine produced a broken
  `/etc/resolv.conf` symlink. The VM's own resolver backup restored Docker Hub
  access. This was a local environment defect, not an application defect.
- `net/smtp` negotiates STARTTLS when the server offers it, but production SMTP
  provider credentials and delivery behavior are not configured or accepted.
- Database and expired-demo object cleanup are implemented in worker order.
  Account deletion is still absent, and an orphan-object reconciler is still
  needed for object writes that succeed before a database transaction fails.
