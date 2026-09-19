# R1 Execution Registry

Status: active execution control
Contract: `../prd/PRD.md`
Plan: `IMPLEMENTATION_PLAN_R1.md`
Last updated: 2026-09-19

## 1. Direct conclusion

R1 implementation may proceed in the independent `supp-q` repository, but RG0
and RG1 cannot pass yet:

- the accountable human owner names and escalation route remain `OPEN_OD_01`;
- the repository was extracted with path history to
  `github.com/harukawakatabe/supp-q` on 2026-09-19;
- `.github/workflows/ci.yml` is active on pushes to `main` and pull requests;
- hosted run `35442408074` passed client, server, and integration E2E at commit
  `55587f9` and retained the integration evidence artifact;
- branch protection and required-check policy remain unconfigured evidence for
  RG1.

Codex may implement and verify changes, but it is not the Product, Security,
QA, or Operations signatory. A single human may hold several roles, but each
role's decision and evidence must be recorded separately.

## 2. Repository and CI decision

| Item | Decision | Consequence |
| --- | --- | --- |
| Active implementation boundary | This repository only | No runtime dependency on frozen `mvp/`, `web/`, `demo/`, or the old `uni/` snapshot |
| Current Git root | `supp-q/` | `app/`, `server/`, `docs/`, and `.github/` are root children |
| Local execution root | Repository root | `make check`, `make test`, `make build`, and integration commands run here |
| Hosted CI | `.github/workflows/ci.yml` | Active on GitHub `main` pushes and pull requests |
| Former parent | `supplement-record/` | Frozen migration source and rollback evidence only |
| RG1 status | `PARTIAL` | Local and hosted execution exist; branch protection and required-check policy remain open |

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
| S0-01 | Document authority and archive map | `DONE_LOCAL` | `DOCUMENT_AUTHORITY.md`; standalone commit `d22437b` |
| S0-02 | Role and responsibility registry | `DONE_LOCAL_WITH_OPEN_OWNER` | This document; actual names remain `OPEN_OD_01` |
| S0-03 | CI/repository-root decision | `DONE_REMOTE` | Extracted to `harukawakatabe/supp-q`; hosted run `35442408074` passed all jobs at `55587f9`; branch-protection evidence remains under RG1 |
| S0-04 | Evidence locations and classification | `DONE_LOCAL` | Section 5 |
| S0-05 | ChangeSpec template | `DONE_LOCAL` | `templates/CHANGE_SPEC.md` |
| S1-01 | Target physical schema specification | `DRAFT_IMPLEMENTATION_SPEC` | `SCHEMA_R1.md`; review pending |
| S1-02 | Current-snapshot inventory queries | `VERIFIED_LOCAL` | `../scripts/sql/r1_source_inventory.sql`; isolated PostgreSQL execution passed |
| S1-03 | Platform-spine expand migration | `VERIFIED_LOCAL` | Standalone implementation commit `d45dae5`; fresh/upgrade/rollback tests passed |
| S1-04 | Fresh/current-snapshot migration rehearsal | `VERIFIED_LOCAL` | Synthetic Launch-Beta fixture; evidence in `../reports/r1/2026-09-19-s1-platform-spine/README.md` |
| S1-05 | Remaining R1 expand/backfill migrations | `NOT_STARTED` | Product/capture/plan/intake/reminder tables after S1-03 review |

State vocabulary:

- `DONE_LOCAL`: artifact exists and local structural checks pass.
- `DONE_REMOTE`: the external repository operation exists and has been directly verified.
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
platform spine VERIFIED_LOCAL at d45dae5
→ accountable engineering/domain review of SCHEMA_R1.md
→ E2 product/profile/ingredient expand migration
→ synthetic current-snapshot backfill and reconciliation
→ compatibility service reads/writes
→ continue E3–E6 only after E2 evidence passes
```

Do not claim S1 complete when only the platform spine passes. S1 closes only
after all required R1 expand/backfill migrations and reconciliation checks pass.
