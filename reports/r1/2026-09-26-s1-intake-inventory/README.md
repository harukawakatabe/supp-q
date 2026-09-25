# S1 E5 Intake-Inventory Local Evidence

Evidence status: `VERIFIED_LOCAL / Synthetic`
Standalone implementation commit: `14b01780ca9b8d6819ab4854c967491971436d5d`
Executed: 2026-09-26 Asia/Shanghai
Environment: Darwin 25.5.0 arm64; Go 1.26.5; Node 25.8.0; local PostgreSQL
Scope: E5 IntakeRecord/InventoryBatch/InventoryEvent/IntakeAllocation metadata,
append-only intake status facts, source/compensation identity, phased catch-up,
compatibility dual writes, Q1 replay, Q2 exact undo, and scoped Q3 intake retry.
Legacy API reads remain authoritative.

## Result

Migration `202609200004_r1_intake_inventory.sql` expands existing facts without
renaming, dropping, or narrowing the legacy read columns. It adds version,
timezone/profile, lifecycle/expiry, source identity, balance snapshot, and
compensation metadata plus tenant-safe relationships and immutable-event
guards.

Current service writes now persist the enriched batch/event/allocation facts in
the same transaction. An intake idempotency key is bound to a SHA-256 hash of
the normalized request; a different request using the same key returns
`idempotency_conflict`. Undo restores the original batches from
IntakeAllocation and links every `intake_undo` event to its exact original
consumption event.

This does not implement correction/supersession commands, inventory adjustment
or batch-void APIs, ClientAction result lookup, target reads, the R2 cost
ledger, or E6 risk/reminder projections. It does not close S1, S7, RG2, RG3,
R1, staging, or production.

## Frozen model decisions

- InventoryEvent is append-only for business changes. Corrections use a new
  event and exact undo uses a unique compensation reference.
- Each allocation records its consumption event, tenant/product ownership,
  allocation mode, batch version, balance before/after, unit, and applicable
  IngredientProfileVersion.
- Legacy current quantity and intake status remain compatibility columns until
  C1. `intake_status_facts` captures the target state chain during E5.
- Batch balance is non-negative but is not capped by original receipt quantity;
  replay and lawful adjustments, not the old ceiling, define correctness.
- Migration does not fabricate unknown ClientActions or request hashes.
- Existing zero `price_cny` remains uninterpreted legacy cost data. E5 records
  currency but does not claim zero cost or create R2 cost layers.

## PostgreSQL and service evidence

`go test ./... -count=1` with `SUPPQ_TEST_DATABASE_URL` passed every server
package. The E5 matrix covers:

- fresh apply and synthetic E4-snapshot upgrade;
- four-phase cursor catch-up, idempotent rerun, attempt/source/hash observation,
  and an N-1 allocation written before its event metadata is linked;
- event/status/allocation immutability checks and cross-product allocation
  rejection;
- stable source uniqueness, batch balance snapshots, allocation sums, tenant
  bindings, and read-only Q1/Q2 reconciliation;
- exact FEFO allocation and original-event compensation;
- eight concurrent identical intake retries producing one intake side effect;
- a different normalized request under the same idempotency key returning a
  conflict;
- eight concurrent undo retries producing one compensation side effect;
- parent-account cleanup, pre-target-write Down, and post-target-write
  SQLSTATE `55000` refusal.

`go vet ./...` passed.

## SQL and client regression evidence

Both `scripts/sql/r1_source_inventory.sql` and
`scripts/sql/r1_reconciliation.sql` executed in read-only transactions through
the migration integration suite. The E5 reconciliation covers replayed batch
balances, per-event balance snapshots, allocation sums/bindings, status-chain
coverage, source uniqueness, and exact compensation.

Client regression remained green even though E5 changes no client code:

- H5 scaffold tests: 7 passed, 0 failed;
- `vue-tsc --noEmit`: pass;
- H5 build: pass, with only the pre-existing Dart Sass legacy-API warnings.

The registry was unreachable for pnpm release verification in this sandbox, so
the client commands used installed pnpm 11.24.0 with
`--pm-on-fail=ignore`; they do not replace the earlier pinned 11.9.0 clean/CI
evidence and do not close RG1.

## Repository and evidence boundary

- Work is on `r1-e2-product-profile`. Per owner instruction, no PR was opened
  and `main` was not changed.
- The workflow runs on pull requests and pushes to `main`; a direct development
  branch push has no hosted E5 CI evidence.
- Data is Fake/Synthetic/local only. No private label image or production data
  was used.
- No E6 migration, complete S7 command/API/UI flow, C1 read cutover, staging,
  production deployment, or production Q1–Q3 claim is made.
