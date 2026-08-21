# H5 Launch-Beta Acceptance

## Main path

```text
anonymous isolated demo
→ inspect seeded Today and Cabinet
→ register through a valid invitation and SMTP code
→ enter an empty registered workspace
→ create manually or upload front/facts/expiry images
→ wait, retry or fall back manually
→ edit and explicitly confirm recognition candidates
→ save product, schedule, and opening batch
→ edit the product and create a new schedule version
→ restock
→ record or backfill an intake
→ verify FEFO allocation
→ undo and restore the exact batches
→ sign out/sign in and survive service restart
→ delete the account and its private objects
```

## Automated evidence

### Go unit and PostgreSQL integration

- Invitation expiry/revocation/use limits and concurrent one-use claim.
- Email-code/password identity binding, reset, logout, session invalidation,
  demo cleanup, exact-email deletion confirmation, durable cleanup claim, and
  final registered-user deletion.
- Demo and registered tenant isolation across products, files, jobs, and
  intakes.
- Weekly/day/long-cycle intersection, independent anchors, non-retroactive
  day-cycle history, fixed-point arithmetic, projected finish/latest start.
- Transactional FEFO split allocation, insufficient-stock rollback,
  idempotency, exact repeat-safe undo, product editing, restock, and record list.
- Private three-role file/job persistence, retry/manual fallback, Fake
  provenance, OCR-before-structure ordering, tenant hiding, idempotent
  confirmation, account object cleanup, and orphan reconciliation.
- Evaluation thresholds reject false-confidence and other failed gates.
- Production configuration rejects insecure origin/transport/SMTP/proxy,
  weak secrets, Fake recognition, and incomplete provider stages.

### Playwright against the real local stack

- Two anonymous browser contexts receive different users, workspaces, and
  seeded product IDs.
- Today, Records, Add, Cabinet, and Me render and navigate on mobile and desktop.
- Manual create → product detail → restock → backfill with note → exact undo.
- Three generated non-private images → durable Fake candidates → edit → human
  confirmation. No product is accepted silently.
- Admin SMTP code → one-use invitation → new registered account → H5 permanent
  deletion submission → new isolated demo.
- Mobile and desktop main pages have no horizontal overflow.
- Browser console/page errors fail the suite; traces, screenshots, videos, and
  Compose logs are retained on CI failure.

### Operational acceptance

- A real restart of PostgreSQL, API, and worker preserves the same session and
  deducted quantity; the script then undoes the acceptance intake and restores
  the original quantity.
- Readiness requires database, object bucket, worker heartbeat, and queue
  access. Metrics report requests, failures, queue state, and heartbeat age.
- Production Compose and Caddy configuration parse; shell scripts pass syntax
  checks; containers rebuild from locked dependencies.

## Recognition quality gate

The application/provider boundary is not accepted from Fake or synthetic-label
connectivity. Use 30–50 explicitly authorized private images covering Chinese
and English fronts, facts panels, expiry dates, glare, blur, crop, and low
contrast. `make recognition-eval INPUT=/absolute/path/to/results.json` must
pass every default threshold:

- field accuracy ≥ 0.90
- correction rate ≤ 0.25
- unrecognized rate ≤ 0.15
- false-confidence rate ≤ 0.05
- provider-failure rate ≤ 0.05
- p95 latency ≤ 45 seconds

The evaluator produces a machine-readable report. A small fixture requires the
explicit `-allow-small` diagnostic flag and cannot close the production gate.

## Production gate

All items require retained evidence:

- DNS and automatic HTTPS work for the selected hostname.
- API and worker preflight reject invalid production configuration.
- Migrations complete before API/worker start.
- PostgreSQL, object bucket, worker heartbeat, queue, and provider failure are
  observable; internal metrics are not public.
- Real SMTP delivery and abuse/rate-limit behavior pass.
- A selected live provider passes the private recognition gate.
- An encrypted off-host backup runs and restores into separate empty drill
  database/bucket targets with a valid checksum manifest.
- The full browser path passes against production-like dependencies.
- Account deletion removes real test-account database rows and private objects.
- Privacy/terms identify actual processors and retention behavior.

Current result: local code acceptance passes; external production acceptance is
open. A healthy endpoint or a provider-shaped response alone never closes it.
