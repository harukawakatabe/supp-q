# Supp Q Coding Agent Entry Point

## Objective

Build a production-oriented, independently movable project for “小补Q / Supp Q”:

```text
anonymous isolated demo
or invited account
→ capture three label images
→ asynchronous recognition
→ human confirmation
→ supplement cabinet and schedule
→ daily intake and undo
→ inventory and expiry risk
→ persistent multi-device account
```

The first shipping surface is H5. The client must use uni-app, Vue 3, and TypeScript so a later WeChat mini-program can reuse platform-neutral application code. The production server is a Go API and worker backed by PostgreSQL and private object storage.

## Required Reading

Read these files before changing code:

1. `PRD.md`
2. `docs/DECISIONS.md`
3. `docs/ARCHITECTURE.md`
4. `docs/PROJECT_STATUS.md`
5. `docs/HANDOFF.md`
6. The task-specific document under `docs/`

## Frozen Areas

- `../web/`
- `../mvp/`
- `../demo/`

Do not modify or import from them. They are historical evidence only. Copy a needed rule or test into `uni/`, record its provenance in the commit or handoff, and let the new copy evolve independently.

## Non-negotiable Boundaries

- All new project files live under `uni/`.
- UI code never accesses the database, provider keys, or object storage credentials.
- Route handlers authenticate, validate, authorize, call application services, and map responses.
- All private queries include the effective `user_id` and `workspace_id`.
- Anonymous demo visitors receive isolated expiring identities; they never share one writable user.
- Demo and registered users use the same domain services and database tables.
- OCR and LLM providers stay behind adapters.
- OCR candidates require user confirmation before becoming final product or schedule data.
- Deterministic schedule, inventory, expiry, and nutrient calculations cannot be replaced by free-form LLM output.
- Model API keys never enter the client bundle, logs, Git, or committed environment files.
- Fake providers are allowed only in development and tests, and must identify themselves as fake.
- An HTTP 200, a queued job, or a provider response is not end-to-end acceptance.
- Documentation must separate `Shipped`, `In progress`, `Planned`, and `Deferred`.

## Definition of Done

A feature is complete only when:

1. Its H5 user path works.
2. Its request and response contract is validated.
3. Persistent state is correct after restart and re-login.
4. Authorization and isolation are tested.
5. Failure, timeout, retry, and recovery states are visible where applicable.
6. Relevant unit, integration, and browser E2E tests pass.
7. `docs/PROJECT_STATUS.md` and `docs/HANDOFF.md` reflect reality.

