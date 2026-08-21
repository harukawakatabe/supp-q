# Coding Agent Handoff

## Current truth

`uni/` is the only active implementation. It is a locally accepted H5 release
candidate with the deterministic supplement loop, invitation identity, private
three-image recognition boundary, Records/product/Me surfaces, account/file
cleanup, production Compose/Caddy, health/metrics, encrypted backup/restore
tooling, evaluation CLI, and Playwright CI.

It is not deployed. Local Compose intentionally uses `fake:development`.
Synthetic Qwen VL → saved text → Kimi evidence from 2026-08-02 proves transport
and ordering only. No authorized 30–50-image private set has passed, and no
production domain, cloud database/bucket, SMTP, monitoring, backup destination,
or provider has been selected.

## Read first

1. `../AGENTS.md`
2. `../CLAUDE.md`
3. `PRD.md`
4. `docs/PROJECT_STATUS.md`
5. `docs/ACCEPTANCE.md`
6. `docs/DECISIONS.md`
7. `docs/DEVELOPMENT_VS_PRODUCTION.md`
8. `docs/EXTERNAL_RESOURCES.md`
9. `docs/SECURITY.md`
10. `contracts/openapi.yaml`

The sibling `web/`, `mvp/`, and `demo/` directories are frozen. Inspect only;
never modify or import them.

## Local start and acceptance

From `uni/`:

```bash
make dev
make status
curl http://127.0.0.1:3000/api/v1/health/ready
make check
make test
make build
make test-integration SUPPQ_TEST_DATABASE_URL='postgres://suppq:suppq_local_only@127.0.0.1:5432/suppq?sslmode=disable'
make test-restart-persistence
cd app && pnpm exec playwright install chromium && pnpm test:e2e
```

The local app is <http://127.0.0.1:3000>; Mailpit is
<http://127.0.0.1:8025>. Bootstrap the local test admin when identity E2E is
needed:

```bash
docker-compose -f deploy/compose.dev.yml exec -T api \
  /app/suppq-admin bootstrap-admin admin@suppq.local
```

`make down` preserves data volumes. Never remove volumes unless the user
explicitly authorizes destroying local data.

## Production path

1. Owner fills `docs/EXTERNAL_RESOURCES.md` choices and stores real values in
   external secret files based on `deploy/*.env.production.example`.
2. Build images and validate the production Compose configuration.
3. Run `/app/suppq-admin validate-config` with the API environment and
   `/app/suppq-admin validate-config --worker` with the worker environment.
4. Authorize and run the private provider evaluation documented in
   `docs/RECOGNITION_EVALUATION.md`; Fake/synthetic results cannot pass.
5. Deploy migration gate, API, worker, H5, and edge. Verify readiness and scrape
   `/metrics` only from the internal network.
6. Run `scripts/backup.sh`, then restore with `scripts/restore-drill.sh` into
   separate targets containing `drill` in both database URL and bucket name.
7. Run the full browser path against production-like email/storage/provider,
   delete the test account, and retain redacted evidence.

## Scope decisions

- Launch beta is H5 only. WeChat upload/login/subscription work is deferred.
- In-app reminders ship; Web Push is deferred.
- Product-level AI explanation was removed from the launch gate by D-021.
- Health-context fields are not collected until a shipped function needs them
  (D-022).
- Automatic invitation email links are not implemented; the admin deliberately
  shares the one-time secret. SMTP login/reset codes are implemented.

## Known traps

- Never expose server/provider keys to the H5 client or commit secret files.
- Never send private images to a live provider without explicit authorization.
- Do not select the previously failing DeepSeek-OCR route without a new measured
  evaluation; its synthetic output was punctuation-only.
- `fake:development` validates orchestration, never accuracy.
- Readiness is stronger than liveness; it includes database, storage, worker,
  and queue checks.
- Product edits must preserve day-cycle history. The service reconstructs that
  history in `ScheduleView`; removing it changes past Today calculations.
- The CI workflow under `uni/.github/` activates only after `uni/` is the
  repository root.

After every material change, update `PROJECT_STATUS.md`, `ACCEPTANCE.md`, the
OpenAPI contract, and this handoff with evidence—not inference.
