# Decision Log

## Confirmed

| ID | Decision | Rationale |
|---|---|---|
| D-001 | All new project files live under `uni/`. | The directory may become an independent repository. |
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

## Pending external choices

- Cloud vendor: Tencent Cloud or Alibaba Cloud Hong Kong.
- Domain and DNS.
- Production email provider.
- Production OCR/vision provider and model.
- Production LLM provider and model.
- Object storage product and region.
- Formal trademark, app-store, social-handle, and domain clearance.
