# ChangeSpec: <short title>

Status: draft
Owner: <role and person>
Requested by: <source>
Date: <YYYY-MM-DD>
Target release: <R1/R2/R3/R4 or later>

## 1. Decision requested

State the exact product or contract change in one paragraph. Do not describe
only a page or implementation technique.

## 2. User problem and evidence

- User/scenario:
- Current failure:
- Evidence:
- Why the confirmed contract is insufficient:

## 3. Scope delta

| Area | Current confirmed behavior | Proposed behavior | Explicitly unchanged |
| --- | --- | --- | --- |
| Product |  |  |  |
| UI/state |  |  |  |
| Data/history |  |  |  |
| API/client |  |  |  |
| Permissions/privacy |  |  |  |
| Provider/cost |  |  |  |

## 4. Contract impact

- Affected PRD sections and acceptance statements:
- R1–R4 sequencing impact:
- New/changed invariant:
- Capabilities hidden, disabled, migrated, or removed:
- Compatibility window and minimum client version:

## 5. Data and migration

- New/changed objects, fields, constraints, and indexes:
- Existing-row mapping and unknown/quarantine rules:
- Backfill batches, cursor, lock budget, and observability:
- Reconciliation queries and pass criteria:
- Rollback/forward-fix point:
- Deletion, retention, export, and backup effect:

## 6. API and failure contract

- Endpoints/events/jobs affected:
- Request, response, version, and idempotency changes:
- Stable errors and recovery actions:
- Old-client behavior:
- Result-unknown/retry behavior:

## 7. UI and design

- Entry points and navigation changes:
- Success, empty, loading, failure, conflict, unavailable, and deletion states:
- Responsive/accessibility implications:
- Design evidence required:

## 8. Security, privacy, provider, and cost

- New data category or destination:
- Authorization/step-up change:
- Provider, region, training, retention, and disclosure:
- Abuse and cross-tenant tests:
- Cost envelope and disable switch:

## 9. Measurement and operations

- Metric definition changes:
- Required logs/metrics/traces without prohibited content:
- Alerts/runbooks/capacity effect:
- Gate impact and evidence owner:

## 10. Test and release plan

- Unit/property/fuzz:
- Migration/integration/contract:
- Browser/accessibility/visual:
- Real-provider or real-data evaluation:
- Staging/canary/rollback:

## 11. Alternatives

| Alternative | Benefit | Cost/risk | Reason rejected or deferred |
| --- | --- | --- | --- |
| No change |  |  |  |
|  |  |  |  |

## 12. Decision record

| Role | Person | Decision | Date | Evidence / conditions |
| --- | --- | --- | --- | --- |
| Product |  |  |  |  |
| Engineering |  |  |  |  |
| QA |  |  |  |  |
| Security/Privacy |  |  |  |  |

No implementation may treat a draft ChangeSpec as approval. After approval,
update the PRD/decision record, implementation plan, Gate checklist, and status
documents in the same change or an explicitly linked follow-up.
