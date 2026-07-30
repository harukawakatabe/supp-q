# V1 Acceptance

## Main path

```text
anonymous demo
→ inspect seeded Today and Cabinet
→ register through a valid invitation
→ verify email
→ enter an empty real workspace
→ upload three images
→ wait for recognition
→ edit and confirm
→ save product, schedule, and batch
→ record intake
→ verify FEFO allocation
→ undo
→ verify exact batch restoration
→ sign out and sign in on another browser
→ verify persistent state
→ delete account
→ verify data and objects are removed
```

## Required automated coverage

- invitation lifecycle and concurrent claim
- password and email-code login
- session rotation and logout
- anonymous demo isolation
- 24-hour inactivity cleanup
- cross-user authorization
- schedule intersection
- FEFO and exact undo
- recognition retry and manual fallback
- unconfirmed OCR never becomes final data
- provider timeout
- restart persistence
- mobile and desktop H5 browser paths

## Recognition evaluation

Create a 30–50-image private set covering:

- Chinese and English labels
- product front
- Supplement Facts
- expiry dates
- glare, blur, crop, and low contrast

Report:

- field accuracy
- user correction rate
- unrecognized rate
- false confident values
- latency
- provider failure rate

## Production gate

- HTTPS works.
- Database migrations are explicit.
- API, worker, DB, and storage health are visible.
- Backups run.
- A backup has been restored.
- Secrets are absent from the client, logs, and Git.
- No fake provider is enabled.
- The complete main path passes against production-like services.

