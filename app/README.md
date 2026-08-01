# Supp Q client

Official uni-app Vue 3 + TypeScript CLI skeleton, pinned from the current
`dcloudio/uni-preset-vue#vite-ts` template on 2026-08-01.

The official template still constrains `vue-i18n` to the unsupported v9 major.
It is retained temporarily for template compatibility and tracked as upgrade
debt; do not interpret the lockfile as a security-support endorsement.

## Commands

```bash
pnpm install
pnpm dev:h5
pnpm type-check
pnpm test
pnpm build:h5
pnpm build:mp-weixin
```

The H5 development server proxies `/api` to `http://127.0.0.1:8080`. Override
the public API base URL with `VITE_API_BASE_URL`; never place a secret in a
client environment variable.

The current Today page is a Phase 0 shell. Its sample content is explicitly
marked as preview data and is not evidence of implemented schedules, inventory,
intake, authentication, or reminder behavior.
