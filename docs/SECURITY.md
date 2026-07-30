# Security Baseline

## Identity and session

- Passwords use a modern password-hashing algorithm; never reversible encryption.
- Email codes and invitation tokens are stored hashed where possible.
- Tokens expire, are single-purpose, and are rate limited.
- Registered sessions use HttpOnly, Secure, SameSite cookies in production.
- Session rotation occurs at login and privilege changes.
- Generic invitations enforce configured use and expiry limits atomically.

## Demo

- No shared writable demo user.
- Each anonymous browser receives an isolated demo identity.
- Demo identities cannot access admin endpoints.
- Demo provider use has quotas.
- Demo profile data is sample-only.
- Demo uploads and records are deleted after 24 hours without activity.
- Login invalidates the demo session and starts cleanup.

## Authorization

- Every private query filters by effective user and workspace.
- Resource IDs are opaque but never treated as authorization.
- Admin checks are separate from authentication.
- Cross-user access has integration and E2E coverage.

## Uploads

- Validate content signature, MIME, extension, and size.
- Store objects privately.
- Serve through authorized endpoints or short-lived signed URLs.
- Strip unsafe filenames and generate opaque object keys.
- Clean failed, abandoned, expired-demo, and deleted-account objects.

## AI and OCR

- Keys remain server-side.
- Provider content is untrusted.
- Model output is validated.
- OCR candidates require confirmation.
- LLM cannot mutate deterministic data directly.
- Fake providers are visibly identified and forbidden in production acceptance.

## Health-context data

- Collect only fields used by shipped functionality.
- Explain purpose and third-party processing before submission.
- Support deletion of database rows, uploads, and derived artifacts.
- Do not place profile data, raw labels, cookies, or full prompts in logs.

## Operations

- Structured logs use request IDs and redacted identifiers.
- Health checks separate API, database, worker, storage, and provider configuration.
- Backups are encrypted and restored in a drill.
- Production secrets remain outside Git and source directories.

