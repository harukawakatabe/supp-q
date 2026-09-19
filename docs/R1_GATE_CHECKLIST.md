# R1 RG0–RG9 Gate Checklist

Status: active; no production gate is currently accepted
Last updated: 2026-09-19

## Status vocabulary

| Status | Meaning |
| --- | --- |
| `OPEN` | Required evidence is missing or failed |
| `PARTIAL` | Some evidence exists, but it does not cover the target R1 artifact/environment |
| `PASS` | Required evidence for the exact release artifact is retained and accepted |
| `N/A` | Not applicable with a written reason and approver; never a silent skip |

Gate evidence must include release ID, commit, artifact digest, environment,
test/data class (`Fake`, `Synthetic`, `Manual`, `Real`), result, timestamp, owner,
and a stable evidence location.

## Current snapshot

| Gate | Current status | Why |
| --- | --- | --- |
| RG0 Contract ready | `PARTIAL` | PRD is confirmed; named owners and target physical schema/OpenAPI/error/migration specifications remain open |
| RG1 Repeatable build | `PARTIAL` | Historical Launch-Beta local checks passed; target R1 and active root CI have not |
| RG2 Migration safe | `PARTIAL` | The platform spine passes fresh/synthetic-current upgrade and rollback; product/capture/plan/intake/reminder backfills, lock rehearsal and production-like evidence remain open |
| RG3 Domain/security correct | `PARTIAL` | Current FEFO/idempotency/tenant foundation exists; complete R1-applicable Q1–Q9 coverage does not |
| RG4 H5 experience | `PARTIAL` | Current Chromium Launch-Beta path exists; target IA/design/accessibility/browser matrix does not |
| RG5 Performance/resilience | `OPEN` | Target load, SLO, failure injection, alert and recovery evidence is absent |
| RG6 External capabilities | `OPEN` | Production SMTP/storage/provider, authorized label gate and processor evidence are absent |
| RG7 Staging accepted | `OPEN` | No production-like target R1 staging evidence or restore drill |
| RG8 Production canary | `OPEN` | Nothing is deployed |
| RG9 Production accepted | `OPEN` | No canary waves or production sign-off |

## RG0 — Contract ready

- [x] Modules 1–19 of `prd/PRD.md` are user-confirmed.
- [x] Document authority and archive map are explicit.
- [x] R1 scope and R2–R4 boundary are explicit.
- [ ] Actual PO, UX, Engineering, Domain/Data, Security/Privacy, ML, QA, and OPS owners are assigned.
- [ ] Target physical schema and constraints are reviewed.
- [ ] Target OpenAPI delta and stable error additions are reviewed.
- [ ] Permission matrix, retention configuration, migration mapping, and rollback point are implementation-ready.
- [ ] Every R1 work item maps to a PRD section, invariant/acceptance, and Gate.

Pass owner: Product + Engineering + QA.

## RG1 — Repeatable build

Local regression evidence on 2026-09-19 used a fresh isolated Compose project:
SeaweedFS retained 232 free slots after its default seven-volume growth, Go
integration passed, and Playwright completed with 8 passed, 2 intentional
skips, and 0 failures. This is Fake/local evidence and does not close RG1.

- [ ] CI runs from the actual repository root with branch protection.
- [ ] Node/pnpm/Go/base images and third-party actions are pinned.
- [ ] Type-check, lint/format, unit, integration, contract, H5 build, and Go builds pass.
- [ ] Secret, dependency, and image scans pass; Critical count is zero.
- [ ] SBOM, commit, build ID, artifact digest, checksums, and logs are retained.
- [ ] The same immutable artifact is eligible for staging promotion.

Pass owner: Engineering + QA.

## RG2 — Migration safe

- [x] Empty database `up` succeeds for the platform-spine migration.
- [x] A representative synthetic current Launch-Beta snapshot upgrades without reset.
- [x] Source inventory includes state/type/timezone/price/job/file anomalies.
- [ ] Backfill is cursor-based, idempotent, resumable, and observable.
- [ ] Lock targets and the ≤15-minute final-delta window pass rehearsal.
- [ ] Product/plan/intake/inventory/cost/file/timezone counts and invariants reconcile.
- [x] OTC/prescription fixture rows are quarantined and individually accounted for.
- [x] Existing identity/catalog/recognition integration paths and pre-target-write Down rollback pass locally.
- [ ] Production-like backup/restore point, old deployed binary compatibility, and real snapshot reconciliation are proven.
- [x] Contract/drop migration remains deferred to a later release.

Current evidence: standalone implementation commit `d45dae5` and
`../reports/r1/2026-09-19-s1-platform-spine/README.md`. All checked items above
are local/synthetic evidence, not staging or production acceptance.

Pass owner: Engineering + Domain/Data + QA.

## RG3 — Domain and security correct

- [ ] Q1 inventory replay is 100% consistent.
- [ ] Q2 exact undo/compensation is 100% correct.
- [ ] Q3 duplicate core side effects are zero.
- [ ] Q4 confirmed cross-tenant access events are zero.
- [ ] Q5 unconfirmed candidates entering product facts are zero.
- [ ] Q8 deletion-completed residual access is zero.
- [ ] Applicable Q9 incompatible-unit silent sums are zero.
- [ ] ClientAction/result-unknown/retry and concurrent version-conflict cases pass.
- [ ] Session, CSRF/Origin/CORS, IDOR, upload, injection, SSRF, abuse and admin step-up tests pass.
- [ ] Logs, metrics, traces, errors and artifacts contain no prohibited content.

Pass owner: Domain/Data + Security/Privacy + QA.

## RG4 — H5 experience

- [ ] R1 pages follow the confirmed Record/Plan/Add/Ingredient/Cabinet information architecture and capability rules.
- [ ] `DESIGN.md` visual direction is implemented through `DESIGN_IMPLEMENTATION_R1.md`.
- [ ] Success, empty, loading, queued, partial, failure, stale, conflict, offline, deleting and degraded states pass.
- [ ] 320 CSS px and 200% text produce no lost content or horizontal main-flow scroll.
- [ ] Keyboard navigation, visible focus, dialog focus return and error targeting pass.
- [ ] Automated accessibility checks and a real VoiceOver or TalkBack main path pass.
- [ ] Current/previous iOS Safari, Android Chrome, desktop Chrome/Edge/Safari and current Firefox support matrix passes.
- [ ] Mobile and desktop visual review records intentional differences from the baseline.

Pass owner: UX + QA.

## RG5 — Performance and resilience

- [ ] Core read/write API, ClientAction, upload, recognition, projection and reminder budgets pass.
- [ ] Web Vitals pass on supported mobile conditions.
- [ ] The initial capacity and large-workspace datasets pass stable and burst tests.
- [ ] DB pool, queue, outbox and worker backlogs recover within defined windows.
- [ ] DB/object/SMTP/provider/worker/network/result-loss failures produce correct degradation and recovery.
- [ ] SLO dashboards, error budgets, alerts, dedupe, runbooks and escalation are exercised.
- [ ] Backup/restore meets RPO≤1h and RTO≤4h in a production-like drill.

Pass owner: Operations + Engineering + QA.

## RG6 — External capabilities

- [ ] Domain/region/processors/retention/training/cost facts are recorded.
- [ ] Real SMTP, private object storage and required production networking pass least-privilege and failure tests.
- [ ] Recognition uses 30–50 explicitly authorized real images with required language/role/quality coverage.
- [ ] Accuracy, correction, unrecognized, false-high-confidence, provider-failure and p95 thresholds all pass.
- [ ] Manual capture remains usable when recognition is refused, disabled or failed.
- [ ] Privacy/terms/provider disclosure and applicable content-identification review are accepted.
- [ ] R3-only AI items remain `N/A for R1` with the capability absent, not fake-enabled.

Pass owner: Recognition/AI + Security/Privacy + Product + QA.

## RG7 — Staging accepted

- [ ] Staging uses the production topology and the same immutable release digest.
- [ ] Production-like migration, target data reconciliation and compatibility pass.
- [ ] Full supported-browser main paths and accessibility pass.
- [ ] Real external dependency smokes and safe synthetic probes pass.
- [ ] Encrypted off-host backup restores into empty drill targets and passes business smoke.
- [ ] Alert delivery, acknowledgement, escalation and recovery pass.
- [ ] The release evidence pack contains no secret, private label, prompt, note or health content.

Pass owner: QA + Operations + Security/Privacy.

## RG8 — Production canary

- [ ] Production DNS/TLS/security headers, secrets, preflight, migration and readiness pass.
- [ ] Wave 0 internal workspace completes the full path and deletion check.
- [ ] Wave 1 contains only 3–5 explicit pilot workspaces and observes at least 24 hours.
- [ ] Core SLO/error budget, queue/projection, provider cost/quality and alerts remain acceptable.
- [ ] There are zero P0/P1 incidents; any incident stops promotion even after auto-recovery.
- [ ] Rollback/feature-disable and incident communication paths are ready.

Pass owner: Product + QA + Operations.

## RG9 — Production accepted

- [ ] Wave 2 reaches 25% of the eligible invitation cohort for at least 72 hours.
- [ ] Wave 3 reaches the complete eligible cohort with seven days of enhanced observation.
- [ ] Error budgets, R1-applicable Q1–Q9 checks, deletion, backups, alerts and external capabilities remain acceptable.
- [ ] The final evidence pack and release sign-off are retained.
- [ ] `PROJECT_STATUS.md`, `HANDOFF.md`, OpenAPI and public status all match the production truth.

Pass owner: Product + QA + Operations + Security/Privacy.

## Promotion rule

A release advances only when every applicable item in the current Gate is
`PASS` or explicitly approved `N/A`. A `PARTIAL` historical result, another
commit's report, Fake/synthetic success, a screenshot, health 200, or a running
container cannot close a Gate.
