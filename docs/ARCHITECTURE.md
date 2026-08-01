# Target Architecture

Status: target design with a locally verified Phase 0 runtime skeleton. Domain
architecture below remains unimplemented unless `PROJECT_STATUS.md` says
otherwise.

## System

```text
Browser / future WeChat mini-program
→ Caddy HTTPS
→ H5 static assets
→ Go API
→ PostgreSQL
→ private object storage
→ persistent job records
→ Go worker
→ OCR / vision provider
→ optional LLM provider
→ email provider
```

## Repository

```text
uni/
├── app/            uni-app client
├── server/         Go API and worker
├── contracts/      OpenAPI, errors, and async event contracts
├── seed/           deterministic demo seed
├── deploy/         local and production deployment assets
├── testdata/       sanitized fixtures and private-sample routing
└── docs/           product, architecture, status, security, and runbook
```

The client was bootstrapped from the official current
`dcloudio/uni-preset-vue#vite-ts` template on 2026-08-01. DCloud packages are
pinned to `3.0.0-5010520260709002`; the lockfile, not this paragraph, is the
dependency source of truth.

The Go server will use one module with multiple commands:

```text
server/
├── cmd/
│   ├── api/
│   ├── worker/
│   └── admin/
└── internal/
    ├── auth/
    ├── invitation/
    ├── product/
    ├── schedule/
    ├── inventory/
    ├── intake/
    ├── recognition/
    ├── ai/
    ├── notification/
    ├── storage/
    └── observability/
```

This follows the Go convention of keeping server packages under `internal` and separate programs under `cmd`.

## Identity

### Registered

```text
user
→ auth_identity[email_password]
→ auth_identity[email_code]
→ future auth_identity[wechat]
→ session
→ workspace
```

### Anonymous demo

```text
no valid session
→ create demo_ephemeral user
→ create demo workspace
→ clone deterministic seed
→ issue HttpOnly demo session
→ update last_activity_at
→ delete after 24h inactivity
```

Registration creates a new registered user and empty real workspace. It invalidates the demo session and schedules the demo identity, workspace, uploads, and generated artifacts for deletion. No demo data is migrated.

## Data ownership

Every private aggregate is scoped by `user_id` and `workspace_id`. Authorization checks happen before domain operations. Object keys include an opaque workspace scope, but object paths alone are not authorization.

Core aggregates:

- users, auth_identities, sessions
- invitations, invitation_acceptances
- workspaces, workspace_members
- profiles
- products, product_labels, ingredients
- schedules, schedule_versions
- inventory_batches, stock_adjustments
- intake_records, intake_allocations
- files, recognition_jobs
- ai_threads or explanation_jobs where needed
- reminder_preferences
- audit_events

## API

Contracts are defined in `contracts/openapi.yaml` when implementation begins.

General response requirements:

- Request ID on every request.
- Stable machine-readable error code.
- User-facing message.
- Validation details without secrets.
- Idempotency key for retryable writes.
- Cursor pagination for growing collections.
- UTC timestamps over the wire; user timezone stored explicitly.

## Jobs

Recognition and AI calls are not synchronous business mutations.

```text
uploaded
→ queued
→ running
→ succeeded | partial | failed | cancelled
→ awaiting_confirmation
→ confirmed
```

Jobs require:

- persisted state
- attempt count
- timeout
- exponential backoff
- provider and model metadata
- creator and workspace scope
- failure code safe for display
- cleanup behavior

V1 may use a PostgreSQL-backed job table and worker. Redis is not mandatory until measured queue pressure justifies it.

## Provider boundaries

Adapters expose explicit provider identity:

- `fake`
- `live:<provider>`

Production refuses to start recognition or AI capability as “available” when required live configuration is absent. Core non-AI flows remain usable where safe.

## Deployment

Development uses PostgreSQL 17.10, SeaweedFS 4.29 as a replaceable local S3
sandbox, Mailpit 1.30.0, and Caddy 2.11.4. Production uses Caddy for HTTPS,
static H5 delivery, and API reverse proxy. PostgreSQL remains the durable source
of truth; production objects live in private Tencent COS or Alibaba OSS; backup
artifacts live outside the application server.

The locally implemented API surface is deliberately narrow:

```text
GET /api/v1/health/live  → process liveness only
GET /api/v1/health/ready → authenticated PostgreSQL ping
```

The worker currently proves process separation and database connectivity only.
It does not poll or execute jobs yet.
