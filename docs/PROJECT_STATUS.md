# Project Status

Last updated: 2026-08-01

## Shipped to production

Nothing in `uni/` is deployed or serving real users yet.

## Implemented and verified locally

- Official current uni-app Vue 3 + TypeScript CLI dependency baseline, limited
  to H5 and future WeChat mini-program targets.
- Notion-like warm-neutral Today shell with explicit Phase 0 preview labeling.
- Responsive mobile bottom navigation and desktop sidebar shell.
- Client API health request with online/offline state.
- H5 and `mp-weixin` builds, TypeScript check, and four client scaffold tests.
- Go 1.26.5 module with independent API, worker, and admin commands.
- Server-generated request IDs, JSON logs, strict configured-origin CORS,
  liveness, and PostgreSQL-backed readiness.
- Eight Go tests across configuration, CORS, liveness, and readiness behavior.
- OpenAPI 3.1 operational contract for implemented health endpoints.
- Pinned local stack: PostgreSQL 17.10, SeaweedFS 4.29, Mailpit 1.30.0,
  Caddy 2.11.4, API, and worker.
- Local S3 credentials and pre-created private `suppq-uploads` bucket.
- Multi-stage non-root Go container and build-tested H5 container.
- Make targets, tool versions, environment examples, and standalone CI file.
- Goose 3.27.1 embedded, append-only identity migration and a migration gate
  that completes before API and worker startup.
- Isolated PostgreSQL-backed `demo_ephemeral` users, workspaces, HttpOnly
  sessions, 24-hour rolling inactivity expiry, and worker cleanup.
- Shared generic-code/email-bound invitation lifecycle with expiry, maximum
  use, single-use email binding, revocation, acceptance rows, and atomic claim.
- Email verification-code login through SMTP/Mailpit, Argon2id password login,
  password reset with all-session invalidation, logout, and two email auth
  identities bound to the same user.
- Minimal H5 authentication and invitation-administration pages. Invitation
  secrets are returned only once; the initial admin is bootstrapped by CLI.
- Stable Phase 1 error registry and OpenAPI 0.2 identity contract.

## Acceptance evidence on 2026-08-01

- `pnpm install --frozen-lockfile`: pass with four explicitly allowed build
  dependencies.
- `pnpm type-check`: pass.
- `pnpm test`: 4/4 pass.
- `pnpm build:h5`: pass.
- `pnpm build:mp-weixin`: pass.
- `go test ./...`: pass with writable isolated build cache.
- `go vet ./...`: pass.
- `go build ./cmd/api ./cmd/worker ./cmd/admin`: pass.
- `docker-compose ... config`: pass.
- All six runtime containers start; PostgreSQL, API, and Mailpit report healthy.
- Direct liveness, direct readiness, and Caddy-proxied readiness return real
  responses; readiness includes `database: ready`.
- Worker logs a successful database heartbeat and explicitly reports jobs as
  `not_implemented`.
- SeaweedFS logs confirm the `suppq-uploads` bucket was created; anonymous S3
  root access returns HTTP 403.
- Mailpit reports v1.30.0 through its live API.
- Browser check at 390x844 confirms API connected, zero horizontal overflow,
  fixed bottom navigation, and no clipped cards. Desktop 1280px breakpoint has
  fixed sidebar navigation and zero horizontal overflow.
- Identity integration test creates its own PostgreSQL schema and passes demo
  isolation, generic/email-bound invitations, concurrent one-use claim,
  email-code plus password login, password reset, session invalidation, and
  demo cleanup.
- Live Mailpit delivery received six-digit codes; a real generic invitation was
  claimed, use count advanced atomically, and the same user re-entered with a
  password in a separate cookie session.
- Live email-bound request returned 400 for the wrong email and 202 for the
  bound email.
- Real H5 browser at 390x844 completed demo → password login → registered empty
  workspace → logout → new isolated demo. Admin login exposed the invitation
  page with both created invitations and no horizontal overflow.

## In progress

- Closing Phase 1 identity documentation and repeatable acceptance commands.
- Preparing Phase 2 deterministic product, schedule, inventory, and intake
  schema without weakening user/workspace authorization boundaries.

## Not started

- Product, schedule, inventory, and intake domains.
- Upload and recognition job implementation.
- OCR/vision and Kimi live/fake provider adapters.
- Main-path H5 interactions beyond the Today shell.
- Application reminders.
- Domain authorization, integration, and browser E2E suites.
- Production deployment, backup, restore, monitoring, and alerting.
- Automatic delivery of email-bound invitation links through the selected
  production email provider.
- Reverse-proxy/IP abuse limits and automated browser E2E for identity paths.
- Demo sample products and records; the identity is seeded only after Phase 2
  domain tables exist.

## Deferred after V1

- H5 Web Push.
- WeChat login and mini-program subscription messages.
- Data export.
- Complete AI conversation and note organization.
- Advanced cost analytics.
- Public registration.
- Payments and plans.

## Known debt and environment notes

- The current official uni-app template constrains `vue-i18n` to unsupported
  v9. It remains pinned for template compatibility and must be evaluated before
  product localization work.
- The uni-app/Vite pipeline calls Dart Sass's deprecated legacy JS API. Builds
  pass, but this must be resolved before Dart Sass 2 adoption.
- The CI file lives under `uni/.github/` and activates only when `uni/` becomes
  its own repository root, preserving the frozen parent boundary.
- The first Colima 0.10.3 VM on this machine produced a broken
  `/etc/resolv.conf` symlink. The VM's own resolver backup restored Docker Hub
  access. This was a local environment defect, not an application defect.
- `net/smtp` negotiates STARTTLS when the server offers it, but production SMTP
  provider credentials and delivery behavior are not configured or accepted.
- Database demo cleanup is implemented. Storage-object cleanup cannot be wired
  until Phase 3 creates file records and an object-storage adapter.
