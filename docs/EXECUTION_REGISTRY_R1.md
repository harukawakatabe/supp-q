# R1 Execution Registry

Status: active execution control
Contract: `../prd/PRD.md`
Plan: `IMPLEMENTATION_PLAN_R1.md`
Last updated: 2026-09-19

## 1. Direct conclusion

R1 implementation may proceed locally, but RG0 and RG1 cannot pass yet:

- the accountable human owner names and escalation route remain `OPEN_OD_01`;
- the current Git repository root is the parent `supplement-record/` directory;
- `uni/.github/workflows/ci.yml` is therefore a portable CI definition, not an
  active GitHub Actions workflow in this checkout;
- the repository boundary will not be changed implicitly during schema work.

Codex may implement and verify changes, but it is not the Product, Security,
QA, or Operations signatory. A single human may hold several roles, but each
role's decision and evidence must be recorded separately.

## 2. Repository and CI decision

| Item | Decision | Consequence |
| --- | --- | --- |
| Active implementation directory | `uni/` only | No new runtime dependency on `mvp/`, `web/`, or `demo/` |
| Current Git root | Parent `supplement-record/` repository | Frozen historical projects remain versioned but are not built |
| Local execution root | `uni/` | `make check`, `make test`, `make build`, and integration commands run here |
| Portable CI definition | `uni/.github/workflows/ci.yml` | It becomes active only when `uni/` is the repository root |
| Root-level workflow | Not created | Creating project CI outside `uni/` would violate the current project boundary |
| RG1 status | `PARTIAL` | Local repeatability can improve; branch protection and active hosted CI remain open |

Before RG1 can pass, the project owner must explicitly choose and execute one
of these repository operations:

1. extract `uni/` into its own repository while preserving relevant history; or
2. approve a contract change that allows a root workflow scoped only to `uni/`.

The default recommendation is option 1 because `uni/` is already designed to
be independently movable and its workflow paths assume it is the root.

## 3. Role registry

| Role ID | Accountable role | Actual person | Current authority | Required sign-off |
| --- | --- | --- | --- | --- |
| PO | Product Owner | `OPEN_OD_01` | Product scope, prioritization, user acceptance | RG0, RG4, RG8, RG9 |
| UX | UX Owner | `OPEN_OD_01` | Information architecture, design adaptation, accessibility review | RG0, RG4 |
| EL | Engineering Lead | `OPEN_OD_01` | Schema/API architecture, migrations, release artifact | RG0–RG3, RG5, RG7 |
| DD | Domain/Data Owner | `OPEN_OD_01` | Schedule, inventory, units, Q1–Q3/Q9 | RG2, RG3 |
| SP | Security/Privacy Owner | `OPEN_OD_01` | Tenant isolation, provider disclosure, retention, deletion | RG0, RG3, RG6–RG9 |
| ML | Recognition/AI Owner | `OPEN_OD_01` | Provider, evaluation data, model/policy change | RG6 |
| QA | QA Owner | `OPEN_OD_01` | Test mapping, evidence integrity, release acceptance | RG0–RG9 |
| OPS | Operations Owner | `OPEN_OD_01` | Infrastructure, alerts, backup/restore, canary | RG5, RG7–RG9 |

Missing names do not stop local schema implementation, but they block formal
Gate acceptance and all production promotion.

## 4. Work register

| Work ID | Scope | State | Evidence / blocker |
| --- | --- | --- | --- |
| S0-01 | Document authority and archive map | `DONE_LOCAL` | `DOCUMENT_AUTHORITY.md`; commit `4035d0a` |
| S0-02 | Role and responsibility registry | `DONE_LOCAL_WITH_OPEN_OWNER` | This document; actual names remain `OPEN_OD_01` |
| S0-03 | CI/repository-root decision | `DECIDED_DEFERRED_ACTIVATION` | Section 2; owner must authorize repository extraction or boundary change |
| S0-04 | Evidence locations and classification | `DONE_LOCAL` | Section 5 |
| S0-05 | ChangeSpec template | `DONE_LOCAL` | `templates/CHANGE_SPEC.md` |
| S1-01 | Target physical schema specification | `DRAFT_IMPLEMENTATION_SPEC` | `SCHEMA_R1.md`; review pending |
| S1-02 | Current-snapshot inventory queries | `DONE_LOCAL` | `../scripts/sql/r1_source_inventory.sql` |
| S1-03 | Platform-spine expand migration | `IMPLEMENTED_UNVERIFIED` | Migration and integration test; status updates after execution |
| S1-04 | Fresh/current-snapshot migration rehearsal | `OPEN` | Requires PostgreSQL-backed test evidence |
| S1-05 | Remaining R1 expand/backfill migrations | `NOT_STARTED` | Product/capture/plan/intake/reminder tables after S1-03 review |

State vocabulary:

- `DONE_LOCAL`: artifact exists and local structural checks pass.
- `IMPLEMENTED_UNVERIFIED`: code exists but required runtime evidence is absent.
- `VERIFIED_LOCAL`: required local runtime tests pass for the exact commit.
- `OPEN`: required decision or evidence is missing.
- `NOT_STARTED`: no implementation claim.

## 5. Evidence locations

| Evidence class | Location | Repository rule |
| --- | --- | --- |
| Safe design/specification evidence | `docs/`, `contracts/` | Commit normally |
| Migration source and read-only checks | `server/migrations/`, `scripts/sql/` | Commit normally |
| Safe release summaries/manifests | `reports/r1/<release-id>/` | May be committed after content review |
| Hosted CI logs and artifacts | CI artifact store | Reference immutable run and digest; do not copy secrets |
| Private label evaluation inputs/results | `testdata/recognition/private/`, `reports/private/` | Git-ignored; never commit |
| Production evidence | Restricted operations store | Record pointer, owner, digest, environment, and timestamp only |

Every retained result must identify commit, artifact digest where applicable,
environment, data class (`Fake`, `Synthetic`, `Manual`, `Real`), result, owner,
timestamp, and skip/N/A reason.

## 6. Current execution rule

The next valid transition is:

```text
S1-03 migration exists
→ fresh database migration test
→ pre-migration current-snapshot fixture
→ upgrade and reconciliation
→ mark S1-03/S1-04 VERIFIED_LOCAL
→ review schema before adding product/plan/capture backfills
```

Do not claim S1 complete when only the platform spine passes. S1 closes only
after all required R1 expand/backfill migrations and reconciliation checks pass.
