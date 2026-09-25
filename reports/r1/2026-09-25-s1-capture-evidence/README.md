# S1 E4 Capture-Evidence Local Evidence

Evidence status: `VERIFIED_LOCAL / Synthetic`
Standalone implementation commit: `1cc7f0cb4bfb5c06a73f9f5f9dbfe25bf5310a07`
Executed: 2026-09-25 Asia/Shanghai
Environment: Darwin arm64; Go 1.26.5; Node 25.8.0; local PostgreSQL
Scope: E4 CaptureDraft/CaptureSlot/SlotVersion, version-bound target recognition
jobs and attempts, FileLinks, immutable evidence/candidates, ConfirmationDraft,
set-level catch-up, compatibility writes, atomic confirmation, and read-only
reconciliation. Legacy recognition reads remain authoritative.

## Result

Migration `202609200003_r1_capture_evidence.sql` adds the E4 shadow capture
model without dropping or narrowing legacy recognition tables. Every mapped
draft has three fixed roles, while missing roles remain empty and receive no
invented file, version, job, attempt, evidence, or candidate.

The implementation does not add independent per-slot upload/replace/skip/manual
APIs or H5 states and does not switch reads to the target model. It does not
close S1, RG2, R1, staging, or production. E5-E6, S4/S5 target surfaces, and C1
target-read comparison/cutover remain open.

## Frozen model decisions

- A target `capture_recognition_jobs` row binds exactly one uploaded
  CaptureSlotVersion. It may reference a legacy Job for compatibility, but the
  legacy `(recognition_set_id, role)` identity is not the target job identity.
- Each lease/retry is a separate `capture_recognition_attempts` row. OCR
  evidence and structured candidates bind the exact target job, attempt, file,
  draft, and SlotVersion.
- Evidence and candidates are append-only. A late result is retained against
  its source attempt but may update an editable ConfirmationDraft only after
  the legacy terminal CAS succeeds and while the SlotVersion remains current.
- Multiple active legacy sets are quarantined rather than silently cancelled.
- Active FileLinks must be released before a file can be marked deleted.
- Migration Down is allowed only while every E4 row is migration-owned. Any
  compatibility/target write or replacement version requires a forward fix.

## PostgreSQL and service evidence

`go test ./... -count=1` with `SUPPQ_TEST_DATABASE_URL` passed every server
package. The E4 matrix covers:

- fresh apply and synthetic E3-snapshot upgrade;
- fixed three-slot mapping with partial roles and no fabricated evidence;
- idempotent rerun and N-1 recognition-set catch-up;
- independent v2 replacement with a target Job and no additional legacy Job;
- immutable target identity, evidence, and candidates;
- active-file protection and parent-account cascade for an isolated Capture
  owner;
- old SlotVersion late-result retention without confirmation merge;
- concurrent lease reclaim where attempt 1 finishes after attempt 2, with the
  ConfirmationDraft still referencing attempt 2;
- atomic confirmation of Product/profile/plan/opening inventory, CaptureDraft,
  ConfirmationDraft, source/media links, and DomainChange;
- pre-target-write Down and post-target-write SQLSTATE `55000` refusal.

`go vet ./...` passed.

## SQL and client regression evidence

Both `scripts/sql/r1_source_inventory.sql` and
`scripts/sql/r1_reconciliation.sql` executed in read-only transactions against
an isolated schema migrated through E4. The E4 fixture also verifies the
non-empty mapping and late-result rules.

Client regression remained green even though E4 changes no client code:

- H5 scaffold tests: 7 passed, 0 failed;
- `vue-tsc --noEmit`: pass;
- H5 build: pass, with only the pre-existing Dart Sass legacy-API warnings.

## Repository and evidence boundary

- Work is on `r1-e2-product-profile`. Per owner instruction, no PR was opened
  and `main` was not changed.
- The workflow runs on pull requests and pushes to `main`; a direct development
  branch push has no hosted E4 CI evidence.
- Data is Fake/Synthetic/local only. No private label image or production data
  was used.
- No independent target Capture API/UI, E5/E6 migration, C1 read cutover,
  staging, production deployment, or real-provider accuracy claim is made.
