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
| `invalid_confirmation` | 400 | Account deletion confirmation does not exactly match the active registered email. |
| `invitation_required` | 400 | A new account did not provide an invitation. |
| `invitation_invalid` | 400 | Invitation is missing, mismatched, expired, revoked, or exhausted. |
| `invalid_invitation_kind` | 400 | Admin supplied an unsupported invitation kind. |
| `invalid_expiration` | 400 | Invitation expiration is not in the future. |
| `invalid_max_uses` | 400 | Generic invitation use limit is outside 1–1000. |
| `unauthorized` | 401 | No active registered session. |
| `forbidden` | 403 | Authenticated actor lacks the admin role. |
| `invitation_not_found` | 404 | Revocation target does not exist or is already revoked. |
| `resource_not_found` | 404 | Tenant-scoped product, batch, intake, recognition set/job, or file is absent; cross-tenant resources are intentionally indistinguishable. |
| `invalid_product` | 400 | Product fields violate the deterministic domain contract. |
| `invalid_schedule` | 400 | Date, weekday, reminder, or cycle rules are invalid. |
| `invalid_batch` | 400 | Batch quantity, price, or expiry is invalid. |
| `invalid_ingredient` | 400 | Ingredient amount or unit is invalid. |
| `invalid_intake` | 400 | Intake date, time, source, quantity, or idempotency key is invalid. |
| `invalid_date` | 400 | Today/record-range dates are invalid, reversed, or exceed 366 days. |
| `insufficient_inventory` | 409 | Full requested intake cannot be allocated; no partial mutation is committed. |
| `invalid_upload` | 400 | Exactly three valid JPEG, PNG, or WebP label images were not supplied within the size limits. |
| `recognition_processing` | 409 | Confirmation was attempted before all recognition jobs reached a terminal state. |
| `recognition_cancelled` | 410 | A cancelled recognition set cannot be confirmed. |
| `rate_limited` | 429 | Email or per-IP API/authentication request limit was reached. |
| `email_delivery_failed` | 502 | SMTP delivery failed; the challenge was invalidated. |
| `storage_unavailable` | 503 | Private object storage could not persist or retrieve an upload. |
| `internal_error` | 500 | Unexpected server or database failure. |

Messages are Chinese user-facing text. Logs use request IDs and retain the
internal error without returning it to the client.

Recognition job failures are persisted as job state rather than returned as
the upload request's HTTP error. Current codes include
`recognition_not_configured`, `provider_unavailable`, `provider_http_error`,
`provider_invalid_response`, `ocr_unavailable`, `ocr_http_error`,
`ocr_invalid_response`, `ocr_unusable`, `structure_unavailable`,
`structure_http_error`, `structure_invalid_response`,
`evidence_persistence_failed`, and `storage_unavailable`. Retryable failures are
requeued up to the stored `maxAttempts`; terminal failures retain the image and
remain manually confirmable.
