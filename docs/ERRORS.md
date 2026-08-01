# Stable Error Registry

Every error response uses:

```json
{"error":{"code":"invitation_invalid","message":"…","requestId":"…"}}
```

| Code | HTTP | Meaning |
|---|---:|---|
| `invalid_json` | 400 | Body is malformed, oversized, or contains unknown fields. |
| `invalid_content_type` | 400 | JSON endpoint did not receive `application/json`. |
| `invalid_email` | 400 | Email normalization or validation failed. |
| `invalid_password` | 400 | Password is outside the 10–256 character policy. |
| `invalid_credentials` | 400 | Email, one-time code, or password is invalid. |
| `invitation_required` | 400 | A new account did not provide an invitation. |
| `invitation_invalid` | 400 | Invitation is missing, mismatched, expired, revoked, or exhausted. |
| `invalid_invitation_kind` | 400 | Admin supplied an unsupported invitation kind. |
| `invalid_expiration` | 400 | Invitation expiration is not in the future. |
| `invalid_max_uses` | 400 | Generic invitation use limit is outside 1–1000. |
| `unauthorized` | 401 | No active registered session. |
| `forbidden` | 403 | Authenticated actor lacks the admin role. |
| `invitation_not_found` | 404 | Revocation target does not exist or is already revoked. |
| `rate_limited` | 429 | Email challenge request limit was reached. |
| `email_delivery_failed` | 502 | SMTP delivery failed; the challenge was invalidated. |
| `internal_error` | 500 | Unexpected server or database failure. |

Messages are Chinese user-facing text. Logs use request IDs and retain the
internal error without returning it to the client.
