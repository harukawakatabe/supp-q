# Decision Log

## Confirmed

| ID | Decision | Rationale |
|---|---|---|
| D-001 | All new project files live in the active project repository. | The former `uni/` directory was extracted as the independent `supp-q` repository on 2026-09-19. |
| D-002 | `web/`, `mvp/`, and `demo/` are frozen. | Prevent split sources of truth and accidental legacy development. |
| D-003 | H5 ships first; WeChat mini-program follows. | Validate the service while keeping cross-platform client constraints. |
| D-004 | Client target is uni-app + Vue 3 + TypeScript. | Reuse platform-neutral application code later. |
| D-005 | Server target is Go + PostgreSQL + private object storage. | Replace local JSON/device identity with production persistence and authorization. |
| D-006 | Generic codes and email-bound invitations share one model. | One lifecycle and audit trail, two claim methods. |
| D-007 | H5 supports password and email-code login. | Provide recovery and lower-friction access. |
| D-008 | Anonymous visitors receive isolated seeded demo identities. | Preserve instant demo value without shared writable data. |
| D-009 | Demo identities expire after 24 hours of inactivity. | Bound storage, provider cost, and privacy exposure. |
| D-010 | Demo changes are not migrated on registration. | Prevent sample data and provisional edits from contaminating real records. |
| D-011 | Registered users do not retain a demo workspace. | Avoid workspace switching and unnecessary product complexity. |
| D-012 | V1 reminders are in-app only. | Web Push and mini-program subscription messages are deferred explicitly. |
| D-013 | V1 AI is product-level explanation and never blocks core flows. | Preserve value while keeping deterministic behavior authoritative. |
| D-014 | V1 includes deletion; export is deferred. | Deletion is a production boundary; export can follow after the schema stabilizes. |
| D-015 | Visual direction is Notion-like warm white/off-white restraint. | Prioritize calm hierarchy and information density over decoration. |
| D-016 | Working brand is 小补Q / Supp Q. | Short and product-relevant; formal brand clearance remains pending. |
| D-017 | UI is Chinese; label recognition supports Chinese and English in V1. | Matches current product scope. |
| D-018 | Development continues with the existing Kimi and vision-provider direction behind adapters. | Reuse known provider capability without coupling contracts or exposing keys; fake providers remain explicit test tools only. |
| D-019 | Live recognition is evidence-first: image transcription is persisted before any structuring request. | Preserve the source text for retries and human review, prevent an unpersisted intermediate from becoming an opaque final candidate, and make provider comparison measurable. |
| D-020 | The currently accepted development route is Qwen VL transcription → persisted text → Kimi structuring; direct VL and dual comparison remain configurable. | A live synthetic-label check on 2026-08-02 produced correct front/facts/expiry transcription and structured facts. The configured DeepSeek-OCR endpoint returned punctuation garbage and is rejected by the text-quality gate; this is not a production-provider acceptance decision. |
| D-021 | The initial H5 launch-beta scope defers product-level AI explanation, superseding D-013 as a release requirement. | The deterministic record/inventory loop is independently valuable; unmeasured model advice would add privacy, safety, evaluation, and provider dependencies without unblocking a core task. |
| D-022 | Health-context fields are deferred until a shipped function has a field-level purpose. | Data minimization is stronger than collecting sensitive context for an unused profile screen. |
| D-023 | Non-H5 upload adapters are not an H5 launch gate. | The first surface is H5; WeChat remains explicitly deferred and must not hold back H5 acceptance. |
| D-024 | The confirmed complete product contract is `prd/PRD.md`; the old Launch-Beta PRD is archived. | Prevent the smaller implemented baseline from silently becoming the final scope. |
| D-025 | `docs/DESIGN.md` is an active user-provided UI direction and must be preserved. | R1 adapts its Clay-inspired visual language through `docs/DESIGN_IMPLEMENTATION_R1.md`; mobile task clarity, accessibility, and page-level user review decide literal application. |
| D-026 | Superseded by D-028: keep the former parent Git root only until explicit repository extraction. | This preserved the frozen legacy boundary while the owner had not yet authorized the repository operation. |
| D-027 | R1 schema migration uses expand/backfill/compatibility/validate/later-contract stages. The first migration adds only the platform spine and quarantine evidence. | Existing Launch-Beta reads/writes must stay functional while immutable versions and projections are reconciled; no destructive cutover is justified yet. |
| D-028 | Extract the complete `uni/` path history into the initially private standalone `harukawakatabe/supp-q` repository; keep the former parent snapshot frozen for rollback. | Makes the portable root workflow active, removes ambiguity about the implementation boundary, and preserves relevant history without importing frozen projects. Visibility is superseded by D-030. |
| D-029 | In local/CI SeaweedFS, cap development volumes at 64 MB and gate API/worker startup on at least 14 free volume slots. | SeaweedFS mini grows seven volumes per collection. A small CI disk auto-sized to seven total slots, then the default collection exhausted them before `suppq-uploads` could allocate its own seven. Bucket existence and merely non-zero free capacity were both false-positive readiness signals. |
| D-030 | The standalone repository is public; development is delivered on `r1-e2-product-profile` without a PR or `main` merge until the owner changes that policy. | Supersedes only the visibility/delivery part of D-028. The current workflow runs on PRs and `main`, so development-branch evidence remains local and must not be reported as hosted CI. |
| D-031 | E3 maps a legacy plan only when reminder times are valid, unique, and equal in count to `dose_times_per_day`; each DoseSlot keeps the full `dose_quantity`. | Averaging, duplicating, or inventing a time would create unsupported dose semantics. Invalid rows are quarantined. Existing day-cycle boundaries are retained, while historically unversioned weekly/long/slot/profile/timezone fields are marked `legacy_current_only`. |
| D-032 | ScheduleVersion binds the immutable ProductProfileVersion and WorkspaceTimezoneVersion; `planStartDate` is a hard lower boundary and inventory never determines plan state, plan day, or occurrence identity. | Unit/timezone interpretation must remain stable, anchors only determine phase, and depletion is an inventory fact rather than a pause. |
| D-033 | E3 occurrence identity is SHA-256 deterministic over ScheduleVersion, local date, and DoseSlot; DST gaps shift forward and folds choose the earlier instant. Migration creates the projection contract but no unbounded horizon. | Rebuilds must preserve identity and produce exactly one instant per local slot. Bounded materialization and target-read cutover remain later service work. |

## Pending external choices

- Cloud vendor: Tencent Cloud or Alibaba Cloud Hong Kong.
- Domain and DNS.
- Production email provider.
- Production OCR/vision provider and model.
- Production LLM provider and model.
- Object storage product and region.
- Formal trademark, app-store, social-handle, and domain clearance.
- Roobert font licensing or the final production font/fallback decision.
