# uni-app Client

Target: uni-app + Vue 3 + TypeScript, H5 first, future WeChat mini-program.

The client has not been bootstrapped yet. Use the official current uni-app CLI template rather than inventing dependency versions. Preserve the standard uni-app project concepts:

- `pages/`
- `static/`
- `App.vue`
- `pages.json`
- `manifest.json`
- shared platform-neutral services and state
- explicit platform adapters for H5 and `mp-weixin`

The client may receive only public configuration such as the API base URL. Provider, database, SMTP, object-storage, and session secrets are server-only.

