# Phase 1 Identity and Invitation

Status: implemented and verified locally on 2026-08-01. Production provider
configuration and automated browser E2E remain pending.

## User paths

### Anonymous demo

`GET /api/v1/session` creates a unique `demo_ephemeral` user, demo workspace,
membership, and HttpOnly session when no valid cookie exists. Activity extends
the expiry by 24 hours. No visitor shares a writable identity.

### Invitation registration

```text
generic code or email-bound secret
→ request email code
→ retrieve the one-time code from email
→ verify code and atomically claim invitation
→ create registered user and empty registered workspace
→ optionally bind password identity
→ rotate session
→ queue originating demo identity for deletion
```

The invitation row is locked during claim. A concurrent integration test proves
that only one claimant can consume a one-use invitation.

### Returning login and reset

- Email-code login resolves the verified `email_code` identity.
- Password login resolves the separate `email_password` identity belonging to
  the same user.
- Password reset consumes a purpose-bound email challenge and invalidates every
  existing session for that user.
- Logout invalidates the server-side session. A later session request creates a
  new isolated demo.

## Secret handling

- Session tokens, invitation secrets, and email codes are stored only as
  purpose-separated HMAC-SHA256 digests.
- Passwords use Argon2id with 64 MiB memory, three iterations, four threads, and
  a random 16-byte salt.
- Invitation plaintext is returned only on creation.
- Production requires a non-development token pepper and Secure cookies.
- Cookies are HttpOnly and SameSite=Lax. State-changing bodies require JSON;
  configured-origin CORS blocks cross-origin credentialed JSON requests.

## Local operation

Mailpit receives local codes at <http://127.0.0.1:8025>.

Bootstrap or promote the first local administrator:

```bash
docker-compose -f deploy/compose.dev.yml exec -T api \
  /app/suppq-admin bootstrap-admin admin@suppq.local
```

Then request an email code through H5, log in, and open **邀请管理**. The
bootstrap command is idempotent and does not set a password; the administrator
may bind one during email-code verification.

## Known gaps

- The demo identity is real and isolated, but sample products and intake records
  cannot be seeded until Phase 2 adds those domain tables.
- Cleanup currently removes database rows by cascade. Object deletion joins the
  same workflow after file records and storage adapters exist.
- Request throttling is per email in PostgreSQL. Production still needs reverse
  proxy/IP rate limits and abuse monitoring.
- Email-bound invitations currently expose a secret for the administrator to
  send. Production provider selection and automatic invitation-email delivery
  remain pending.
- WeChat is a reserved `auth_identities.provider` value only. No WeChat login or
  binding UI is shipped.
- Browser acceptance is currently manual and evidence-backed; a repeatable H5
  browser E2E suite is still required before deployment.
