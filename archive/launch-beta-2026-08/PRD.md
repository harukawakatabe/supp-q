# 小补Q / Supp Q PRD

Status: H5 launch-beta scope and product baseline
First surface: H5  
Future surface: WeChat mini-program  
UI language: Chinese  
Supported label languages in V1: Chinese and English

## 1. Product definition

小补Q is a personal supplement record service that turns label images into user-confirmed product data and schedules, then supports daily intake records, inventory, and expiry risk.

It is not a diagnosis or prescription service. OCR, translation, and AI explanations are candidates or supporting explanations. The user-confirmed label and deterministic business rules remain authoritative.

## 2. V1 outcome

```text
anonymous isolated demo
or invitation-based account
→ upload product front, facts, and expiry images
→ wait for recognition
→ review and edit candidates
→ save a product, schedule, and opening inventory
→ see today's plan
→ record, backfill, or undo an intake
→ update the exact inventory batch
→ see low-stock and expiry risk
→ sign in on another device and retain data
```

## 3. Users and access

### 3.1 Anonymous demo

- A visitor without a registered session automatically receives an isolated `demo_ephemeral` user and demo workspace.
- The workspace is seeded with sample products. Intake history starts empty.
- Each visitor has separate data; there is no shared writable demo account.
- Demo and registered users use the same domain services and persistence model.
- Demo data expires after 24 hours without activity.
- Login or registration ends the anonymous demo session.
- Demo changes are not migrated into the registered account.
- The registered account starts with an empty real workspace.
- Demo uploads and generated data are removed with the expired demo identity.

### 3.2 Invitations

One invitation model supports:

- Generic invitation codes.
- Email-bound invitation links.
- Expiration.
- Maximum uses for generic codes.
- Single use for email-bound invitations.
- Revocation.
- Acceptance audit.
- Email verification before account activation.

### 3.3 Authentication

H5 V1 supports:

- Email and password.
- Email verification code.
- Password reset.
- Multiple authentication identities bound to one user.

The schema reserves a future WeChat identity without pretending that WeChat login is shipped.

### 3.4 Administration

A minimal admin page supports:

- Creating a generic code.
- Creating an email-bound invitation.
- Viewing invitation status.
- Revoking invitations.
- Inspecting expiration and acceptance.

## 4. V1 scope

### P0

- H5 responsive application.
- Isolated anonymous demo.
- Invitation, registration, login, logout, password reset, and email-code login.
- Three-image capture or upload.
- Asynchronous OCR/vision recognition.
- Visible queued, processing, partial, failed, retry, and confirmation states.
- Manual entry and editing.
- Supplement cabinet.
- Weekly, day-cycle, and long-cycle schedule intersection.
- Product batches and FEFO allocation.
- Daily intake, ad hoc intake, backfill, and exact undo.
- Low-stock, projected finish, latest-start, and expiry risk.
- In-app reminder state and summaries.
- Account data deletion and associated file cleanup.
- Minimal invitation administration.

### Deferred

- User profile and health context until a shipped function has a defined need
  for each sensitive field.
- Product-level AI explanation. Launch-beta value and safety do not depend on
  model-generated advice; label recognition remains candidate-only.
- H5 Web Push.
- WeChat mini-program subscription messages.
- WeChat login and identity binding UI.
- User data export.
- Complete product and ingredient AI conversation migration.
- AI note organization.
- Advanced cost ledger and analytics.
- Public self-service registration.
- Payments, plans, and quotas beyond safety limits.

Deferred work must remain visible in `docs/PROJECT_STATUS.md`.

## 5. Information architecture

### Public

```text
Anonymous demo Today
└── Authentication
    ├── Login
    ├── Invitation acceptance
    └── Password reset
```

### Authenticated workspace

Mobile bottom navigation:

```text
Today
Records
Add
Cabinet
Me
```

Desktop H5 uses the same entries in a left sidebar.

### Today

- Today's scheduled products.
- Done and pending state.
- Quick intake and undo.
- Backfill and ad hoc intake.
- In-app reminder summary.
- Low-stock and expiry risk summary.

### Records

- Recent daily intake records.
- Backfill and undo history.

### Add

```text
choose input
→ product front
→ supplement facts
→ expiry
→ recognition jobs
→ confirmation
→ schedule
→ opening batches
→ save
```

### Cabinet

- Active, paused, and depleted products.
- Product overview and editing.
- Schedule and reminder-time editing.
- Batches, restock, inventory, and ingredients.

### Me

- Account identity, privacy copy, and deletion.
- Invitation administration for admins.
- Reminder summary and product-detail entry.
- Privacy and data deletion.
- Future export entry marked unavailable until implemented.

## 6. Visual direction

The visual system is inspired by Notion-like restraint and Nokno's warm editorial style:

```css
--canvas: #f7f6f3;
--surface: #ffffff;
--surface-muted: #f1f1ef;
--text: #37352f;
--text-secondary: #787774;
--text-muted: #9b9a97;
--border: #e9e5df;
--action: #37352f;
--action-hover: #2f2e2a;
--success: #448361;
--warning: #c58a24;
--danger: #c4554d;
--recognition: #6b65a8;
```

Rules:

- Warm off-white canvas, white working surfaces.
- Continuous lists before card grids.
- Dark neutral primary actions.
- Green means completion or safety, amber means review, red means danger.
- Recognition may use restrained purple.
- No neon gradients, glassmorphism, dense dashboard walls, or decorative motion.
- High contrast and visible keyboard focus are required.

## 7. Product rules

- Recognition never silently becomes final product or schedule data.
- Inventory allocation is FEFO.
- Undo restores the exact original batch allocations.
- Weekly, day-cycle, and long-cycle conditions are intersected.
- Rest days do not consume inventory.
- LLM output cannot change deterministic quantities.
- Fake provider output is never shown as live recognition.
- Registered users cannot access another user's products, files, jobs, records, or profile.

## 8. V1 success criteria

- The complete P0 path passes browser E2E.
- A second account cannot access the first account's resources.
- Anonymous demo visitors do not share state.
- Demo data and files are removed after 24 hours of inactivity.
- Provider failure retains the user's upload and presents retry/manual entry.
- Restarting API and worker does not lose permanent user state.
- Backup restore is exercised before production launch.
