# S1 E3 Product-Plan Local Evidence

Evidence status: `VERIFIED_LOCAL / Synthetic`
Standalone implementation commit: `fcc9dda079312e6aea9eb4a03d04d8be43310cf8`
Executed: 2026-09-20 Asia/Shanghai
Environment: Darwin arm64; Go 1.26.5; Node 25.8.0; PostgreSQL 17.10
Scope: E3 ProductPlan/ScheduleVersion/DoseSlot/PlanStateInterval expand,
backfill, compatibility writes, deterministic occurrence evaluator, and
read-only reconciliation. Legacy Product/Today schedule reads remain
authoritative.

## Result

Migration `202609200002_r1_product_plans.sql` adds the E3 shadow plan model and
strengthens WorkspaceTimezoneVersion tenant identity and immutability. Manual
create, recognition confirmation, and product update write legacy and E3 plan
facts in one transaction. Only a real plan-field change appends a target
ScheduleVersion; product metadata changes and repeated identical PUTs do not.

The implementation does not switch reads to E3, does not materialize an
unbounded occurrence horizon, and does not close S1, RG2, R1, staging, or
production. E4-E6 and C1 target-read comparison/cutover remain open.

## Frozen migration decisions

- `plan_start_date` is a hard lower boundary. Cycle anchors determine phase and
  cannot create earlier plan days.
- Weekly, day-cycle, and long-cycle rules are intersected. Inventory never
  participates in plan-day or occurrence identity.
- A legacy schedule is automatically mapped only when every reminder is a
  valid unique `HH:MM` value and reminder count equals `dose_times_per_day`.
  Each slot keeps `quantity=dose_quantity`; unsafe rows are quarantined rather
  than averaged or padded.
- Existing day-cycle boundaries become immutable ScheduleVersions. Weekly,
  long-cycle, slot, profile, and timezone values unavailable historically are
  copied conservatively and marked `legacy_current_only`.
- `active` and `depleted` map to an active plan; `paused` and `archived` map to
  paused intervals. The migration observation instant is recorded without
  pretending it is the historical pause instant.
- Occurrence ID is deterministic SHA-256 over
  `scheduleVersionID|localDate|doseSlotID`, shaped as a UUID with version nibble
  5. DST gaps shift forward by the gap; folds choose the earlier instant.

## PostgreSQL evidence

The migration package passed on a real local PostgreSQL 17.10 instance:

- fresh apply through `202609200002`;
- synthetic current Launch-Beta snapshot upgrade;
- mismatch quarantine without an empty ProductPlan;
- one-row batch cursor, cross-connection resume, completed-cycle rerun, and
  N-1 legacy drift append;
- historical day-cycle mapping, active/paused interval mapping, tenant-safe
  profile/timezone links, immutable schedule/state/slot bodies, non-overlapping
  intervals, and deterministic occurrence IDs;
- Product parent cascade cleanup;
- pre-target-write Down with E1 timezone FK restoration;
- post-target-write Down refusal with SQLSTATE `55000`.

`go test ./... -count=1` with `SUPPQ_TEST_DATABASE_URL` passed every server
package, including catalog, recognition, identity, evaluator, and migrations.
`go vet ./...` passed.

## Evaluator and compatibility evidence

- UTC, Asia/Shanghai, New York DST gap/fold, cross-year cycle, effective
  interval, hard plan start, three-layer AND, disabled-cycle, multi-slot, and
  repeated-ID tests passed.
- Create produces one ProductPlan, initial ScheduleVersion, DoseSlots, and an
  active PlanStateInterval bound to the current ProductProfileVersion and
  WorkspaceTimezoneVersion.
- Name-only update creates no target schedule version. A schedule edit appends
  once; repeating the same update appends nothing.
- Pause/resume changes only PlanStateInterval. Depletion, intake, restock, and
  undo do not change target plan state or schedule versions.
- Recognition confirmation uses the same transactional create path.
- Existing legacy Product and Today reads were not redirected.

## SQL and client regression evidence

Both `scripts/sql/r1_source_inventory.sql` and
`scripts/sql/r1_reconciliation.sql` ran successfully in read-only transactions
against an isolated schema migrated through E3. All empty-schema critical
violation counts were zero. Non-empty mapping, quarantine, drift, interval,
tenant, cascade, deterministic-ID, and rollback behavior is covered by the
synthetic PostgreSQL fixtures. The temporary schema was dropped; development
data and volumes were not removed.

Client regression remained green:

- H5 scaffold tests: 7 passed, 0 failed;
- `vue-tsc --noEmit`: pass;
- H5 build: pass, with only the pre-existing Dart Sass legacy-API warnings.

## Repository and evidence boundary

- Repository visibility was read from the GitHub API on 2026-09-20 as
  `public`; default branch remains `main`.
- Work is on `r1-e2-product-profile`. Per owner instruction, no PR was opened
  and `main` was not changed.
- The workflow runs on pull requests and pushes to `main`; therefore a direct
  development-branch push has no hosted E3 CI evidence.
- Data is Fake/Synthetic/local only. No private label image or production data
  was used.
- Branch protection/ruleset enforcement has not been configured and verified;
  RG1 remains partial.
