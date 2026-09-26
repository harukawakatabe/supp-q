# S1 E6 Risk-Reminder Local Evidence

Evidence status: `VERIFIED_LOCAL / Synthetic`
Standalone implementation commit: `0a54f1e21a97e66c8791dc3b1e5f1b654915bfca`
Executed: 2026-09-26 Asia/Shanghai
Environment: Darwin arm64; Go 1.26.5; Node 25.8.0; local PostgreSQL
Scope: E6 deterministic risk evaluator and complete-revision persistence,
explicit in-app reminder preference versions, Product overrides, event/target
lifecycle, dedupe, monotonic materialization cursors, conservative migration,
and compatibility copy correction. Legacy API reads remain authoritative.

## Result

Migration `202609200005_r1_risk_reminders.sql` adds the E6 target facts without
renaming, dropping, or narrowing current API columns. One risk revision cannot
be activated until it contains exactly one Product result and the declared
number of batch results. Activating a complete replacement supersedes the prior
active revision in the same Product-scoped transaction.

Every migrated Workspace receives only a `needs_confirmation` ReminderPreference
aggregate with no version. Existing `reminder_times[]` and risk thresholds do
not become authorization and do not generate events. A confirmed preference is
created or replaced through one version-checked transaction that closes the
prior immutable version and writes all four window facts. The stored channel is
restricted to `in_app + not_required`.

The target reminder event model persists source/policy identity, a
Workspace-unique dedupe key, immutable fact snapshots, independent read time,
guarded lifecycle transitions, tenant-bound targets, and cursor positions that
cannot regress. Product overrides only support `inherit` or `muted`; there is no
`force_on` path around a global choice.

The new target-domain risk evaluator uses stable FEFO order, excludes already
expired inventory from automatic allocation, keeps unknown expiry last,
simulates occurrence quantities, finds the first shortfall, separates stock and
expiry states, and retains month/year date ranges. It is not yet wired into
legacy Product/Today reads.

## Local verification

`go test ./... -count=1` with `SUPPQ_TEST_DATABASE_URL` passed every server
package against local PostgreSQL. The E6 migration matrix covers:

- launch-snapshot upgrade and conservative, idempotent N-1 Workspace catch-up;
- incomplete revision activation refusal, complete activation, replacement
  supersession, and post-activation result refusal;
- atomic preference version replacement and exact four-window persistence;
- event dedupe, independent read state, terminal transition refusal, and
  in-app-only channel checks;
- monotonic cursor advancement and backward-position refusal;
- parent-account cleanup, pre-target-write Down, and post-target-write
  SQLSTATE `55000` refusal.

Risk unit tests cover stable FEFO, expired exclusion, unknown-expiry behavior,
first shortfall, low stock, month precision boundaries, paused-plan expiry
progression, unfinishable batches, and invalid input refusal. `go vet ./...`
passed. Both E1-E6 source-inventory and reconciliation SQL scripts executed in
read-only transactions.

Client regression passed:

- H5 scaffold tests: 8 passed, 0 failed;
- `vue-tsc --noEmit`: pass;
- H5 build: pass, with only the pre-existing Dart Sass legacy-API warnings.

The registry was not used for pnpm release verification in this sandbox. The
client commands used the installed pnpm 11.24.0 with `--pm-on-fail=ignore`;
they do not replace the earlier pinned 11.9.0 clean/CI evidence and do not close
RG1.

## Copy and safety boundary

The H5 copy now calls legacy `reminderTimes` values “计划时点” and explicitly
states that they are not notification authorization. The account page does not
claim that an event center or external notification is available. E6 adds no
Web Push, WeChat, email, SMS, device token, provider delivery, prescription/OTC
entry, or AI medication advice.

## Remaining work

This E6 foundation does not implement the Worker or request-time materializer,
DomainChange consumption, risk/reminder APIs, target-read shadow comparison,
reminder center/settings UI, deep-link behavior, browser E2E, or real-time
cross-device convergence. It does not close S8, C1, RG1-RG9, R1 staging, or
production acceptance.

Work remains on `r1-e2-product-profile`. Per owner instruction, no PR is used
and `main` is not changed. Evidence is synthetic/local only; no private label
image or production user data was used.
