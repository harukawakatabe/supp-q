# R1 Execution Registry

Status: active execution control
Contract: `../prd/PRD.md`
Plan: `IMPLEMENTATION_PLAN_R1.md`
Last updated: 2026-09-26

## 1. Direct conclusion

R1 implementation may proceed in the independent `supp-q` repository, but RG0
and RG1 cannot pass yet:

- the accountable human owner names and escalation route remain `OPEN_OD_01`;
- the repository was extracted with path history to
  `github.com/harukawakatabe/supp-q` on 2026-09-19;
- `.github/workflows/ci.yml` is active on pushes to `main` and pull requests;
- hosted run `35453181201` passed client, server, and integration E2E at main
  commit `04538ce` and retained artifact digest
  `sha256:25db16dbc8743ad7126889e7faef39cd3447e37b6c286e428d82a1b9af0519c7`;
- the GitHub API reported the repository as `public` on 2026-09-20;
- the owner selected direct development-branch delivery without a PR. Because
  CI triggers only pull requests and pushes to `main`, the E2/E3/E4/E5 development
  branch has local evidence but no hosted branch run;
- branch protection/ruleset enforcement has not been configured and verified.
  RG1 enforcement therefore remains open.

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
| Repository visibility | `public` | Verified through the GitHub API on 2026-09-20 |
| Delivery policy | Direct development branch; no PR; do not update `main` | Branch work needs explicit local evidence because current CI does not run on ordinary development-branch pushes |
| RG1 status | `PARTIAL` | Main has historical hosted CI; current branch has local evidence only and branch enforcement is unverified |

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
| S0-03 | CI/repository-root decision | `DONE_REMOTE` | Public `harukawakatabe/supp-q`; hosted main run `35453181201` passed at `04538ce`; direct development-branch/no-PR policy is active; enforcement remains under RG1 |
| S0-04 | Evidence locations and classification | `DONE_LOCAL` | Section 5 |
| S0-05 | ChangeSpec template | `DONE_LOCAL` | `templates/CHANGE_SPEC.md` |
| S1-01 | Target physical schema specification | `DRAFT_IMPLEMENTATION_SPEC` | `SCHEMA_R1.md`; review pending |
| S1-02 | Current-snapshot inventory queries | `VERIFIED_LOCAL` | `../scripts/sql/r1_source_inventory.sql`; isolated PostgreSQL execution passed |
| S1-03 | Platform-spine expand migration | `VERIFIED_LOCAL` | Standalone implementation commit `d45dae5`; fresh/upgrade/rollback tests passed |
| S1-04 | Fresh/current-snapshot migration rehearsal | `VERIFIED_LOCAL` | Synthetic Launch-Beta fixture; evidence in `../reports/r1/2026-09-19-s1-platform-spine/README.md` |
| S1-05 | E2 Product/Profile/Ingredient expand, backfill, reconciliation, compatibility writes | `VERIFIED_LOCAL` | Implementation `e667a08`; evidence in `../reports/r1/2026-09-20-s1-product-profile/README.md` |
| S1-06 | E3 ProductPlan/ScheduleVersion/DoseSlot/PlanStateInterval expand, backfill, evaluator, reconciliation, compatibility writes | `VERIFIED_LOCAL` | Implementation `fcc9dda`; evidence in `../reports/r1/2026-09-20-s1-product-plan/README.md`; old schedule reads remain authoritative |
| S1-07 | E4 Capture/SlotVersion/RecognitionJob/Attempt/Evidence/Candidate expand, backfill, reconciliation, compatibility writes | `VERIFIED_LOCAL` | Implementation `1cc7f0c`; evidence in `../reports/r1/2026-09-25-s1-capture-evidence/README.md`; old recognition reads remain authoritative |
| S1-08 | E5 Intake/Batch/Event/Allocation expand, catch-up, reconciliation, compatibility writes | `VERIFIED_LOCAL` | Implementation `14b0178`; evidence in `../reports/r1/2026-09-26-s1-intake-inventory/README.md`; legacy reads remain authoritative and full S7 is open |
| S1-09 | E6 risk/reminder expand group | `NOT_STARTED` | Continue only from the verified E5 boundary; revision/dedupe/retry evidence required |

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
→ E2 Product/Profile/Ingredient VERIFIED_LOCAL at e667a08
→ E3 ProductPlan/ScheduleVersion VERIFIED_LOCAL at fcc9dda
→ E4 Capture/Evidence VERIFIED_LOCAL at 1cc7f0c
→ E5 Intake/Inventory VERIFIED_LOCAL at 14b0178
→ push the owner-selected development branch without PR or main merge
→ continue E6 only after E5 evidence remains green
```

Do not claim S1 complete when only the platform spine passes. S1 closes only
after all required R1 expand/backfill migrations and reconciliation checks pass.
