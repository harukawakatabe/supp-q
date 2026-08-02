# V1 Acceptance

## Main path

```text
anonymous demo
→ inspect seeded Today and Cabinet
→ register through a valid invitation
→ verify email
→ enter an empty real workspace
→ upload three images
→ wait for recognition
→ edit and confirm
→ save product, schedule, and batch
→ record intake
→ verify FEFO allocation
→ undo
→ verify exact batch restoration
→ sign out and sign in on another browser
→ verify persistent state
→ delete account
→ verify data and objects are removed
```

## Required automated coverage

- invitation lifecycle and concurrent claim
- password and email-code login
- session rotation and logout
- anonymous demo isolation
- 24-hour inactivity cleanup
- cross-user authorization
- schedule intersection
- FEFO and exact undo
- recognition retry and manual fallback
- unconfirmed OCR never becomes final data
- provider timeout
- restart persistence
- mobile and desktop H5 browser paths

## Current Phase 1–3 evidence

Implemented automated coverage:

- two independent demo users and workspaces
- generic and email-bound invitation validation
- concurrent claim against a one-use invitation
- email-code and password identities resolving to the same user
- password reset and all-session invalidation
- login-origin demo cleanup
- schedule intersection, independent anchors, historical day-cycle rules,
  projected finish, latest start, and fixed-point quantity arithmetic
- FEFO split allocation, atomic insufficient-stock rejection, idempotent intake,
  exact repeat-safe undo, Today projection, and cross-tenant resource hiding
- three-role private upload persistence and tenant-scoped set/file reads
- persisted recognition job claiming, explicit Fake provenance, retained
  provider failures, manual fallback, retry, and idempotent confirmation
- proof that recognition candidates create no product before human confirmation
- proof that OCR/VL transcription is committed before Kimi structuring and a
  failed evidence write prevents the structuring request
- rejection of empty or punctuation-only transcription before downstream cost

Manually exercised against the real local stack:

- SMTP delivery into Mailpit
- generic invitation creation and claim
- email-bound wrong-email rejection
- mobile H5 demo, password login, registered workspace, logout, and new demo
- role-gated invitation administration page
- proxied multipart upload to private local object storage, three worker job
  completions, authorized byte-for-byte file delivery, unauthenticated 401, and
repeat-safe confirmation returning one product
- non-private synthetic Qwen VL → persisted transcription → Kimi processing
  for front, facts, and expiry, with provider/model/timing traces visible

This is not the complete V1 acceptance gate. Expired-demo object cleanup is
wired but account deletion, automated browser E2E, restart persistence, backup
restore, and production-like provider behavior remain unaccepted. The Fake
provider validates orchestration and safety boundaries only. The synthetic
live run validates connectivity and evidence ordering only; it is not private
real-label accuracy evidence.

## Recognition evaluation

Create a 30–50-image private set covering:

- Chinese and English labels
- product front
- Supplement Facts
- expiry dates
- glare, blur, crop, and low contrast

Report:

- field accuracy
- user correction rate
- unrecognized rate
- false confident values
- latency
- provider failure rate
- OCR-to-structure disagreement and direct-VL disagreement on a bounded dual subset

## Production gate

- HTTPS works.
- Database migrations are explicit.
- API, worker, DB, and storage health are visible.
- Backups run.
- A backup has been restored.
- Secrets are absent from the client, logs, and Git.
- No fake provider is enabled.
- The complete main path passes against production-like services.
