# Coding Agent Handoff

## Current truth

This repository is the only active implementation. The complete product contract is
confirmed, and `docs/IMPLEMENTATION_PLAN_R1.md` is the current execution plan.
The R1 platform-spine migration at standalone implementation commit `d45dae5` is locally
verified on fresh and synthetic current snapshots; remaining R1 schema groups
and every production Gate are still open.
The code is still a locally accepted H5 release candidate with the deterministic supplement loop, invitation identity, private
three-image recognition boundary, Records/product/Me surfaces, account/file
cleanup, production Compose/Caddy, health/metrics, encrypted backup/restore
tooling, evaluation CLI, and Playwright CI.

It is not deployed. Local Compose intentionally uses `fake:development`.
Synthetic Qwen VL → saved text → Kimi evidence from 2026-08-02 proves transport
and ordering only. No authorized 30–50-image private set has passed, and no
production domain, cloud database/bucket, SMTP, monitoring, backup destination,
or provider has been selected.

## Read first

1. `AGENTS.md`
2. `docs/DOCUMENT_AUTHORITY.md`
3. `prd/PRD.md`
4. `docs/IMPLEMENTATION_PLAN_R1.md`
5. `docs/R1_GATE_CHECKLIST.md`
6. `docs/EXECUTION_REGISTRY_R1.md`
7. `docs/SCHEMA_R1.md` for schema/domain work
8. `docs/DESIGN.md` and `docs/DESIGN_IMPLEMENTATION_R1.md` for UI work
9. `docs/PROJECT_STATUS.md`
10. `docs/DECISIONS.md`
11. `docs/ARCHITECTURE.md`
12. `docs/DEVELOPMENT_VS_PRODUCTION.md`
13. `docs/EXTERNAL_RESOURCES.md`
14. `docs/SECURITY.md`
15. `contracts/openapi.yaml`

The archived Launch-Beta/product-generation documents under `archive/` are
historical evidence only. Frozen `web/`, `mvp/`, `demo/`, and pre-extraction
`uni/` snapshots remain only in the former parent repository; never import or
depend on them.

## Local start and acceptance

From the repository root:

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

`app/package.json` and hosted CI pin `pnpm@11.9.0`. On 2026-09-19, a clean
standalone clone completed `make install`, `make check`, `make test`, and
`make build` through the pinned package-manager path. Hosted run `35442408074`
then passed client, server, and integration E2E at commit `55587f9`.

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

- R1 is H5 only. WeChat upload/login/subscription work is R4.
- In-app reminders ship; Web Push is deferred.
- Product-level AI was removed from the Launch-Beta gate by D-021 and is now an
  R3 controlled supplement-information capability, not a hidden R1 feature.
- Health-context fields remain absent until an R3/R4 shipped function has a
  field-level purpose and authorization (D-022).
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
- In local Compose, `object-storage` uses 64 MB development volumes and must
  report at least 14 free SeaweedFS volume slots before API or worker starts.
  Mini mode grows seven volumes per collection; bucket existence or merely one
  free slot does not prove that the upload collection can allocate a batch.
- Product edits must preserve day-cycle history. The service reconstructs that
  history in `ScheduleView`; removing it changes past Today calculations.
- `.github/workflows/ci.yml` is active in the standalone repository. Keep pnpm
  installation before `actions/setup-node` while pnpm caching is enabled.

After every material change, update `PROJECT_STATUS.md`,
`R1_GATE_CHECKLIST.md`, the implemented OpenAPI contract, and this handoff with
evidence—not inference.
