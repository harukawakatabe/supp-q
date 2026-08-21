# Project Status

Last updated: 2026-08-21

## Direct conclusion

`uni/` is a locally verified H5 release candidate, not a live production
service. The code now contains the launch-beta product loop, deployment
composition, health/metrics, cleanup, backup/restore tooling, and automated
acceptance. Production promotion is blocked by owner-provided infrastructure
and a real-label provider evaluation, not by a hidden Fake-provider claim.

Confidence:

- **High** — local deterministic domain, identity, private-file orchestration,
  H5 main path, restart persistence, production configuration guards.
- **Medium** — production deployment assets and encrypted backup/restore
  scripts are syntax/config validated but have not run against selected cloud
  resources.
- **Unknown** — production recognition accuracy, email delivery, backup
  restore, DNS/TLS, and alert delivery until real services are selected and
  accepted.

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

- PostgreSQL migrations through `202608210001`; six-decimal quantities,
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
  recognition queue; `/metrics` exposes request, queue, and worker gauges for
  an internal collector.
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

## Deferred after launch beta

- Product-level AI explanation (D-021).
- Health-context profile fields until used by a shipped function (D-022).
- H5 Web Push, WeChat login/upload/subscription messages, and mini-program UI.
- Data export, AI note/conversation organization, advanced analytics, public
  registration, payments, and quotas beyond safety limits.

## Known debt

- The official uni-app template still pins `vue-i18n` v9 for compatibility.
- The uni-app/Vite pipeline emits Dart Sass legacy API deprecation warnings.
- Automatic email-bound invitation delivery is not implemented; the admin UI
  returns a one-time secret for deliberate sharing. Login/reset codes do use
  SMTP.
- Production backup tooling depends on `pg_dump`/`pg_restore`, MinIO `mc`, and
  `age`; those tools and real destinations are external operational resources.
- `uni/.github/workflows/ci.yml` activates only when `uni/` is an independent
  repository root, preserving the frozen parent-project boundary.
