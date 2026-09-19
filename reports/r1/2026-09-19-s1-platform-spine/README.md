# S1 Platform-Spine Local Evidence

Evidence status: `VERIFIED_LOCAL / Synthetic`
Standalone implementation commit: `d45dae5`
Executed: 2026-09-19 Asia/Shanghai
Environment: Darwin 25.5.0 arm64; Go 1.26.5; Node 25.8.0; PostgreSQL 17.10
Scope: S1-02, S1-03, and synthetic-local portion of S1-04 only

## Result

The expand-only R1 platform spine applies to a fresh schema and to a synthetic
Launch-Beta snapshot, preserves the legacy rows, records unsupported Product
types in quarantine, enforces the initial command/projection constraints, and
rolls back safely before target writes exist.

This is not proof that S1, RG2, R1, staging, or production is complete.

## Migration test

Command class:

```text
SUPPQ_TEST_DATABASE_URL=<local PostgreSQL> go test ./migrations \
  -run TestR1PlatformSpineFreshAndCurrentSnapshotUpgrade -count=1 -v
```

Result: PASS.

- Fresh schema applied through `202609190001`.
- Synthetic current snapshot first applied through `202608210001`, then seeded
  one workspace, supplement/OTC/prescription rows, product schedule/ingredient,
  one batch, one intake/allocation, and opening/intake ledger events.
- Upgrade retained 3 Products, 1 Schedule, batch balance 8, 2 InventoryEvents,
  1 active Intake and its allocation.
- Exactly one legacy timezone version/pointer was created for the workspace.
- OTC and prescription rows produced 2 open quarantine records; the supplement
  produced none.
- Invalid ClientAction hash and active projection without activation time were
  rejected.
- Down to `202608210001` retained legacy rows and removed the new pointer/tables.

Observed migration execution in the complete integration run was approximately
41 ms for fresh and 56 ms for the synthetic snapshot. These are wall-clock
times on a tiny local fixture, not production lock-time evidence.

## Compatibility evidence

The PostgreSQL-backed integration target passed after adding the migration:

- migrations: PASS;
- identity and account lifecycle: PASS;
- catalog, schedule, FEFO, intake and exact undo: PASS;
- recognition persistence, worker, evidence, confirmation and isolation: PASS.

The complete Go unit suite and command builds passed with local loopback
permission. `go vet ./...` passed.

The existing H5 toolchain was invoked directly from installed `node_modules`:

- `vue-tsc --noEmit`: PASS;
- Node scaffold tests: 7 PASS;
- H5 build: PASS with existing Dart Sass legacy-API deprecation warnings.

The normal `make check` path did not start type-check because pnpm 11.24.0 tried
to switch to the project-declared 11.9.0 and could not verify/download registry
signatures in the restricted environment. No dependency was changed. This is
retained as RG1 toolchain/CI debt, not reported as a source failure.

## SQL evidence

Both committed read-only scripts executed successfully against a separate empty
verification database migrated through `202609190001`:

- `scripts/sql/r1_source_inventory.sql`;
- `scripts/sql/r1_reconciliation.sql`.

All critical reconciliation counts were zero and all six platform tables were
present. Empty-database success does not replace the synthetic snapshot test or
a future real production-like snapshot rehearsal.

## Open evidence

- Actual human owner names and escalation route (`OPEN_OD_01`).
- Branch protection and required-check enforcement; hosted CI is active in the
  standalone repository.
- Real/current production-like snapshot inventory and migration rehearsal.
- Contention and lock-time measurement at representative scale.
- E2–E6 Product/profile, plan, capture, intake/inventory, risk/reminder expand and
  backfill migrations.
- Backup/restore point, old deployed binary compatibility, staging and canary.
