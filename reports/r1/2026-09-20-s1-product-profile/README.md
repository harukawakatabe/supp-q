# S1 E2 Product-Profile Local Evidence

Evidence status: `VERIFIED_LOCAL / Synthetic`
Standalone implementation commit: `e667a08540ac7ca3dea18b0471ca2f83b7858a18`
Executed: 2026-09-20 Asia/Shanghai
Environment: Darwin arm64; Go 1.26.5; Node 25.8.0; PostgreSQL 17.10
Scope: E2 Product/Profile/Ingredient expand, backfill, reconciliation, and
compatibility writes; plus the E1 new-workspace timezone initialization gap

## Result

Migration `202609200001_r1_product_profiles.sql` adds the R1 ProductProfile and
IngredientProfile version model, media/deletion support tables, nullable Product
and batch pointers, a durable batched catch-up cursor, and safe compatibility
writes while legacy reads remain authoritative. It does not switch reads to the
target model and does not close S1, RG2, R1, staging, or production.

OTC and prescription Product rows remain unsupported: existing rows are
quarantined and hidden from ordinary service paths, and new API creation is
rejected. No prescription-management entry or personalized medication advice
was introduced.

## Migration and constraint evidence

The PostgreSQL-backed migration suite passed all E2 cases:

- fresh schema apply;
- synthetic current-snapshot upgrade with three supported and two unsupported
  Products;
- one-row batches persisted and resumed through the durable cursor;
- unchanged reruns created no duplicate versions or items;
- N-1 legacy name/brand/unit/serving/ingredient drift appended the next legacy
  snapshot, including identical values recreated under new ingredient row IDs;
- an existing target catalog state was not overwritten;
- current pointers, tenant relationships, effective intervals, mapping states,
  immutability, and batch bindings were enforced;
- direct history deletion and post-activation item insertion were rejected,
  while Product/User/Workspace cascade cleanup remained functional;
- pre-target-write Down succeeded and retained legacy rows; Down refused once a
  real target write existed.

The migration applied through `202609200001` in 26.71 ms on a separate empty
local schema. This timing is only a tiny local observation and is not
representative lock-budget evidence.

## Compatibility evidence

For the exact implementation commit:

- `go test ./... -count=1` with local PostgreSQL: PASS;
- `go vet ./...`: PASS;
- identity integration with Products present during registered-account and
  expired-Demo cleanup: PASS;
- catalog create/update/add-batch/intake, recognition confirmation, tenant
  isolation, target versioning, and unsupported-type isolation: PASS;
- direct `vue-tsc --noEmit`: PASS;
- H5 scaffold tests: 7 passed, 0 failed;
- H5 build: PASS, with only the pre-existing Dart Sass legacy-API warnings.

## SQL evidence

`scripts/sql/r1_source_inventory.sql` and
`scripts/sql/r1_reconciliation.sql` executed in read-only transactions against
a separate schema migrated through `202609200001`.

- all 11 required platform/E2 tables were present;
- Product/Profile coverage, catalog mapping, unsupported target facts,
  ProductProfile field drift, IngredientProfile serving/raw collection drift,
  batch bindings, tenant relationships, interval overlaps, legacy
  `confirmed_empty`, inventory replay, and all earlier critical checks returned
  zero violations on the empty verification schema;
- the synthetic migration fixture separately proved non-empty mapping,
  quarantine, cursor, drift, binding, and rollback behavior.

The exact temporary schema `suppq_e2_verify_20260920` was dropped after
verification. Development PostgreSQL data and volumes were not removed.

## Review record

Independent contract and acceptance reviews initially found and blocked:

- immutable triggers that also blocked parent cascade cleanup;
- post-activation item insertion;
- catalog-state overwrite on catch-up rerun;
- missing N-1 legacy drift catch-up and incomplete field reconciliation;
- an unsafe unconditional Down after target writes;
- an incomplete ingredient mapping-state enum.

All were corrected and covered by the passing regression suite before the
implementation commit was created.

## Evidence boundary and open work

- Data is Fake/Synthetic/local only. No private label image or production data
  was used.
- Main baseline commit `04538ce` has hosted CI run `35453181201` passing client,
  server, and integration E2E with artifact digest
  `sha256:25db16dbc8743ad7126889e7faef39cd3447e37b6c286e428d82a1b9af0519c7`.
  Hosted PR CI for this E2 branch is separate evidence.
- GitHub reported that repository rulesets are not enforced for this private
  personal-account repository without moving to a GitHub Team organization;
  RG1 branch-protection enforcement therefore remains open.
- C1 target-read cutover, FK/NOT NULL validation, representative-scale lock
  rehearsal, real snapshot reconciliation, backup/restore, and E3-E6 remain
  open.
