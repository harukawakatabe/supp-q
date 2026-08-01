# Contracts

## Implemented

- `openapi.yaml` — Phase 0 liveness, readiness, request-ID header, and error
  envelope contract.

The checked-in API contract is intentionally smaller than the PRD. A path is
not part of the service merely because the PRD describes it.

## Planned

- `errors.md` — complete stable domain error registry.
- `events.md` — persisted background-job events and state transitions.
- identity, invitation, demo, product, schedule, inventory, intake,
  recognition, AI, and reminder paths in `openapi.yaml` before implementation.

Error codes, idempotency, pagination, job states, and time formats are part of
the contract. Client and server must not independently invent them.
