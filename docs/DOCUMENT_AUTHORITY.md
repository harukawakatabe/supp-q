# Document Authority and Archive Map

Status: active
Last updated: 2026-09-19

## Direct conclusion

The product contract is complete. The active product source of truth is
`prd/PRD.md`; the archived Launch-Beta PRD, shaping files, and PRD-generation
logs are evidence of how the decision was reached, not competing instructions.

`docs/DESIGN.md` is an active user-provided UI direction. It must not be
archived or overwritten. `docs/DESIGN_IMPLEMENTATION_R1.md` records how that
visual language is adapted to the R1 mobile H5 product.

## Required reading order

| Order | Document | Authority |
| --- | --- | --- |
| 1 | `prd/PRD.md` | Complete confirmed product contract, scope, rules, data, NFRs, and acceptance |
| 2 | `docs/IMPLEMENTATION_PLAN_R1.md` | Current R1 implementation sequence and work packages |
| 3 | `docs/R1_GATE_CHECKLIST.md` | Current RG0–RG9 release evidence checklist |
| 4 | `docs/DESIGN.md` | User-provided visual direction and design inspiration |
| 5 | `docs/DESIGN_IMPLEMENTATION_R1.md` | Product-specific UI adaptation and visual acceptance contract |
| 6 | `docs/PROJECT_STATUS.md` | Implemented, verified, planned, and production status |
| 7 | `docs/ARCHITECTURE.md` | Current implemented architecture baseline; cannot narrow the PRD |
| 8 | `docs/DECISIONS.md` | Technical and phased-delivery decisions |
| 9 | `contracts/openapi.yaml` | Implemented API contract only, not the complete target API |
| 10 | Task-specific security, identity, error, environment, external-resource, recognition, and handoff docs | Supporting implementation contracts |

## Conflict rules

1. A user-confirmed decision and `prd/PRD.md` override historical product text.
2. `IMPLEMENTATION_PLAN_R1.md` controls execution order but cannot remove R2–R4
   from the complete product scope.
3. `DESIGN.md` controls visual intent. PRD accessibility, state clarity, safety,
   and mobile behavior override literal website-style examples when they conflict.
4. `ARCHITECTURE.md`, `openapi.yaml`, and current code prove what exists. They do
   not define the maximum product scope.
5. `PROJECT_STATUS.md` controls claims about implementation and production. A
   completed PRD or a healthy service is not production acceptance.
6. Archived files never override active files, even when an archived path uses
   words such as “confirmed”, “target”, or “acceptance”.

## Active document set

| Area | Active documents | Maintenance rule |
| --- | --- | --- |
| Product | `prd/PRD.md` | Change confirmed behavior only through a reviewed ChangeSpec |
| R1 delivery | `IMPLEMENTATION_PLAN_R1.md`, `R1_GATE_CHECKLIST.md` | Update work/gate status with evidence, not expectation |
| UI | `DESIGN.md`, `DESIGN_IMPLEMENTATION_R1.md` | Preserve the user source; evolve the adaptation spec with visual review |
| Current truth | `PROJECT_STATUS.md`, `HANDOFF.md`, `README.md` | Update after every material implementation or release change |
| Architecture | `ARCHITECTURE.md`, `DECISIONS.md`, `DEVELOPMENT_VS_PRODUCTION.md` | Separate current implementation from approved target |
| Security/data | `SECURITY.md`, `IDENTITY.md`, `ERRORS.md`, `EXTERNAL_RESOURCES.md` | Keep aligned with implemented controls and current provider choices |
| Recognition | `RECOGNITION_EVALUATION.md` | Never replace real authorized evaluation with Fake/synthetic evidence |
| API | `contracts/openapi.yaml` | Contains implemented endpoints only; target changes land with code |

## Archive map

| Archive | Contents | Why archived |
| --- | --- | --- |
| `archive/launch-beta-2026-08/` | Old `PRD.md` and H5 Launch-Beta acceptance | Correct for the old release candidate, but incomplete and misleading as the current full product contract |
| `archive/product-shaping-2026-09/` | PM-Make intake, JTBD, scope, flows, open questions, shaped brief, parity, visual baseline, data delta | Product shaping is complete and incorporated into the confirmed PRD |
| `archive/prd-generation-2026-09/` | Make-PRD source notes, module plan, and append log | Generation workflow is complete; records remain for audit only |

The archive is recoverable Git history, not a deletion area. Historical paths
inside archived files are preserved as written and may no longer resolve from
their new directory.

## Frozen sibling projects

`../mvp/`, `../web/`, and `../demo/` remain in place because they contain frozen
historical code, tests, and visual evidence. They are not active documentation
or runtime dependencies and must not be edited or imported without an explicit
scope change.

## Update discipline

- Add new implementation files under `uni/` only.
- Do not create another PRD, roadmap, acceptance document, or design system when
  an active document above can be versioned.
- A new document must state its authority, status, owner role, and relationship
  to the active set.
- When a document becomes historical, move it under `archive/`, update every
  active reference, and record the reason here.
- Never archive `DESIGN.md` merely because an implementation differs. Record the
  adaptation and rationale in `DESIGN_IMPLEMENTATION_R1.md`.
