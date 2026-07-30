# 小补Q / Supp Q

`uni/` is the new production project for a supplement label recognition, scheduling, intake, inventory, and reminder service.

## Current status

Only product decisions, architecture boundaries, handoff rules, and directory ownership have been established. The uni-app and Go runtimes have not yet been bootstrapped. No production feature is shipped.

See:

- `PRD.md` — product truth
- `docs/PROJECT_STATUS.md` — shipped versus planned
- `docs/ARCHITECTURE.md` — target architecture
- `docs/DEVELOPMENT_VS_PRODUCTION.md` — local and production differences
- `docs/EXTERNAL_RESOURCES.md` — resources the owner must prepare
- `docs/HANDOFF.md` — next-agent entry point

## Independence rule

This directory must remain movable as a standalone repository. It must not import, read, or execute files from sibling `web/`, `mvp/`, or `demo/` directories.

## Working brand

- Chinese: 小补Q
- English: Supp Q
- Status: working name, not a confirmed trademark

An initial web search did not find an obvious same-name consumer supplement application. “SuppQ” is used elsewhere as an abbreviation for a supplier qualification database, so formal trademark, app-store, social-handle, and domain checks are still required before brand investment.

