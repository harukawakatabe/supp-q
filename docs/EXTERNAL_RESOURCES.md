# External Resources Checklist

Status values: `not selected`, `selected`, `configured`, `verified`.

## Owner-provided resources

| Resource | Development | Production placement | Current status |
|---|---|---|---|
| Domain | localhost | Tencent/Alibaba DNS console | not selected |
| H5 hostname | localhost | DNS + Caddy config | not selected |
| API hostname | localhost | DNS + Caddy config | not selected |
| Hong Kong server | local machine | Tencent or Alibaba lightweight server | not selected |
| PostgreSQL | verified 17.10 container | server container initially or managed DB later | local verified; production not selected |
| Object storage | verified SeaweedFS S3 sandbox | private COS or OSS bucket | local verified; production not selected |
| Email | verified Mailpit 1.30.0 | verified SMTP/email API provider | local verified; production not selected |
| OCR/vision | fake or live test adapter | server-side provider secret | existing provider intended, exact config pending |
| LLM | fake or live test adapter | server-side provider secret | existing Kimi intended, exact config pending |
| Monitoring | local logs | selected logging/alert destination | not selected |
| Backup target | disposable | external bucket or separate backup destination | not selected |
| Admin identity | seed placeholder | owner-controlled verified email | not provided |
| Brand clearance | working name | trademark/domain/app-store checks | pending |

## Required production values

Do not paste secret values into project documents.

### Server and DNS

- Cloud vendor.
- Region and server identifier.
- Public IP.
- H5 hostname.
- API hostname.
- DNS ownership.
- SSH public-key access.

### PostgreSQL

- Host and port.
- Database name.
- Application user.
- Migration user if separated.
- TLS requirements.
- Backup destination and retention.

### Object storage

- Endpoint.
- Region.
- Private bucket.
- Access key and secret.
- Allowed MIME types and size limits.
- Lifecycle policy.
- Backup or replication policy.

### Email

- Provider.
- Verified sending domain.
- Sender name and address.
- SMTP/API endpoint.
- Credentials.
- Invitation, code, and reset templates.

### OCR/vision

- Provider and API protocol.
- Base URL.
- Model.
- API key.
- input size limits.
- timeout and quota.
- data-retention terms recorded by the owner.

### LLM

- Provider and API protocol.
- Base URL.
- Model.
- API key.
- token limits.
- timeout and quota.

### Administrator

- Initial verified email.
- Recovery method.
- Who may create and revoke invitations.

## Placement

| Value | Repository | Local machine | Production |
|---|---|---|---|
| Variable names | `.env.example` | n/a | n/a |
| Public API URL | client public config | `.env.local` | build/deploy environment |
| Provider secrets | never | server `.env.local` | secret file/manager |
| Database credentials | never | server `.env.local` | secret file/manager |
| SSH private key | never | user keychain/SSH directory | not copied into app |
| Uploads | never | SeaweedFS volume | private COS/OSS |
| Private test images | never | `testdata/recognition/private/` | not deployed |
| Deployment facts without secrets | `docs/EXTERNAL_RESOURCES.md` | same | same |

## Production verification

A resource is not `verified` until:

- credentials work from the intended runtime
- least-privilege access is confirmed
- failure behavior is tested
- logs do not expose secrets
- backup or cleanup behavior is exercised
