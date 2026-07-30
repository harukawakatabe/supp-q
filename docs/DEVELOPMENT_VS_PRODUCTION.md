# Development vs Production

## Environment matrix

| Concern | Local development | Production |
|---|---|---|
| Client | uni-app H5 dev server | Built static H5 served by Caddy |
| API | local Go API | supervised Go API container/process |
| Worker | local Go worker | independent supervised worker |
| Database | Docker PostgreSQL | production PostgreSQL with external backups |
| Files | local MinIO | private Tencent COS or Alibaba OSS |
| Email | Mailpit | verified production email provider |
| OCR/vision | explicit fake or live test provider | live provider only |
| LLM | explicit fake or live test provider | live provider; core flow remains independent |
| HTTPS | optional localhost | mandatory |
| Sessions | local secure-enough settings | Secure, HttpOnly, SameSite cookies and rotation |
| Data | deterministic seed and fixtures | real user data |
| Logs | console JSON | retained structured logs and alerts |
| Monitoring | local health output | API, worker, DB, queue, storage, provider metrics |
| Backup | disposable | scheduled encrypted backup and restore drill |
| Demo cleanup | short intervals allowed in tests | 24 hours without activity |
| Reminder | in-app | in-app for V1 |

## Configuration placement

### Committed

- `.env.example` with names and safe descriptions.
- non-secret local defaults.
- compose templates.
- Caddy template using placeholders.
- migration files.

### Never committed

- passwords
- API keys
- cookie/session secrets
- SMTP credentials
- cloud access keys
- production database URLs
- real user data
- private recognition samples

### Local

Use ignored `.env.local` files or shell environment variables. The client receives only public configuration such as the API base URL.

### Production

Secrets live outside the Git checkout in a root-readable environment file, container secret, or cloud secret manager. Runtime data and backups live outside the source directory.

## Behavior differences that are allowed

- Fake providers in development, visibly identified.
- Faster demo expiration in automated tests.
- Mail capture instead of real delivery.
- Local HTTP instead of production HTTPS.

## Behavior differences that are forbidden

- Skipping authorization in development.
- Returning fake recognition without a fake label.
- Using different schedule or inventory rules.
- Replacing PostgreSQL with client localStorage.
- Disabling migrations or idempotency.
- Treating a production provider outage as success.

