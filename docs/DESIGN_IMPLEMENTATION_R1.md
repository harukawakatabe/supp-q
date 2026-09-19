# R1 Design Implementation Contract

Status: draft implementation contract derived from the user-provided design
Source: `DESIGN.md`
Product constraints: `../prd/PRD.md` sections 4 and 18
Last updated: 2026-09-19

## 1. Role of DESIGN.md

`DESIGN.md` is an active visual direction, not an obsolete research note. Its
warm cream canvas, oat borders, tactile surfaces, named swatches, generous
radii, visible focus, and playful craft character are the desired identity.

It is not copied literally from a marketing website. Small-screen H5 task
completion, dense supplement facts, risk semantics, touch, reduced motion,
Chinese typography, and accessibility take priority when a literal example
would make the product harder or less safe to use.

## 2. Keep, adapt, and reject

| Source idea | R1 decision | Product adaptation |
| --- | --- | --- |
| Warm cream `#faf9f7` canvas | Keep | Default app background |
| Oat `#dad4c8` / `#eee9df` borders | Keep | Lists, cards, separators, inputs and evidence panels |
| Matcha/Slushie/Lemon/Ube/Pomegranate/Blueberry palette | Keep with semantic limits | Assign stable roles; do not change the meaning of risk/status colors per page |
| 24px cards / 40px large sections | Adapt | 16–24px cards on mobile; 24–40px only for large containers |
| Dashed borders | Adapt | Empty slots, upload drop zones and secondary evidence; never primary errors/actions |
| Multi-layer tactile shadow | Keep lightly | Level 1 surfaces; preserve contrast and scrolling performance |
| `rotateZ(-8deg) translateY(-80%)` hover | Restrict | Desktop pointer-only delight on non-destructive secondary controls; never touch, primary submit, delete, intake or confirmation |
| 80px/60px display type | Reject for task screens | Reserve 32–44px for rare onboarding/empty hero; task headings remain compact |
| Roobert + stylistic sets | Conditional | Use only with verified web/app licensing and Chinese fallback; never block R1 |
| 6.4px button padding example | Reject as touch specification | All primary touch targets are at least 44×44 CSS px |
| Full-width colored story sections | Adapt | Use for onboarding/Demo explanation, not every operational page |
| Uppercase tracked labels | Adapt | English technical labels only; do not force uppercase Chinese |

## 3. Semantic palette

The swatch palette provides identity; semantic status remains stable and must
not depend on color alone.

| Token | Draft value | Role |
| --- | --- | --- |
| `--canvas` | `#faf9f7` | App background |
| `--surface` | `#ffffff` | Primary working surface |
| `--surface-muted` | `#f5f1e9` | Secondary rows/panels |
| `--border` | `#dad4c8` | Default structure |
| `--border-light` | `#eee9df` | Subtle separation |
| `--text` | `#171714` | Primary text |
| `--text-secondary` | `#55534e` | Secondary text |
| `--text-muted` | `#77736b` | Tertiary text that still passes contrast for its size |
| `--action` | `#02492a` | Primary action/selected state |
| `--action-soft` | `#dff5e6` | Selected/positive background |
| `--info` | `#01418d` | Informational state |
| `--recognition` | `#43089f` | Recognition/AI process, never health safety |
| `--warning` | `#9d6a09` | Review/low-stock/near-expiry |
| `--danger` | `#a8323a` | Destructive/expired/error |
| `--focus` | `#146ef5` | 2px visible keyboard focus |

Matcha does not mean “medically safe”; it means completed/available/successful
within the product's deterministic semantics. Ube is reserved for recognition
and later AI provenance. Lemon and Pomegranate must include icons/text labels.

## 4. Typography

| Role | Mobile target | Desktop target | Notes |
| --- | --- | --- | --- |
| App/page title | 28–32px / 600 | 32–40px / 600 | Tight but not billboard scale |
| Section title | 20–24px / 600 | 24–28px / 600 | One clear hierarchy step |
| Card/row title | 16–18px / 600 | 16–20px / 600 | Product name may wrap |
| Body | 16px / 400 | 16–18px / 400 | 1.5–1.6 line height |
| UI/control | 15–16px / 500 | 15–16px / 500 | Never below 14px for core actions |
| Caption/meta | 13–14px / 400–500 | 13–14px | Must still pass contrast |
| Numeric emphasis | 20–28px / 600 | 24–32px / 600 | Quantities/costs use tabular numbers when available |

Preferred family is licensed Roobert when available. The required fallback for
Chinese H5 is a system stack such as `-apple-system, BlinkMacSystemFont,
"PingFang SC", "Microsoft YaHei", sans-serif`. Space Mono may be used for
opaque IDs or technical evidence, not general product copy.

## 5. Spacing, radius, and elevation

- Base spacing is 4px with primary steps 8/12/16/24/32.
- Mobile page gutter is 16px; desktop content uses a constrained readable width.
- Primary working cards use 16–24px radius; inputs/buttons use 10–14px.
- Pills are reserved for filters, state chips and compact selectors.
- Long product lists prefer continuous rows with subtle oat dividers rather
  than every row becoming a floating card.
- Shadow Level 1 is subtle and used sparingly. Sticky navigation and modal
  elevation must remain clear at 200% zoom.

## 6. Component set

| Component | Required states |
| --- | --- |
| Button | default/hover(pointer)/pressed/focus/disabled/loading/destructive |
| Field | empty/filled/focus/error/read-only/disabled/help/conflict |
| ProductRow | default/paused/no-stock/risk/archived/stale projection/loading |
| StatusChip | text + icon + stable semantic color; no color-only meaning |
| CaptureSlot | empty/uploading/queued/running/partial/succeeded/failed/stale/skipped/manual |
| EvidencePanel | collapsed/expanded/source mismatch/replaced/private-access-failed |
| AsyncPanel | waiting/progress/result-unknown/retry/cancel/degraded |
| OccurrenceRow | pending/partial/completed/exceeded/missed/blocked-no-stock |
| BatchRow | available/depleted/voided/unknown-expiry/near-expiry/expired/unfinishable |
| ReminderItem | available/read/resolved/superseded/cancelled |
| EmptyState | first-use/no-results/filter-empty/unavailable/deferred |
| ErrorState | field/page/dependency/session/conflict/fatal with recovery action |
| ConfirmationDialog | clear consequence, cancel, focus trap/return, busy/result-unknown |

## 7. R1 page groups

| Page group | Visual job | Clay-language use | Constraint |
| --- | --- | --- | --- |
| Demo/Auth | Explain value and identity boundary | Larger swatch panels and playful illustration are allowed | Demo/real data distinction remains dominant |
| Record/Today | Fast low-attention action | Cream canvas, compact white rows, one Matcha primary action | State, quantity, time and inventory must scan before decoration |
| Add/Capture | Make three evidence roles understandable | Dashed oat slots, Ube recognition state, evidence panels | Independent slot state and manual fallback always visible |
| Confirm | Compare candidate and evidence | White structured sections, restrained conflict color | One primary “confirm and add” action; no automatic facts |
| Cabinet/Product | Browse and inspect | Continuous rows, tactile detail cards, stable risk chips | Search/filter/state density before colorful storytelling |
| Plan | Explain layered schedules | Color bands/timeline may separate cycle layers | Colors do not replace labels or version dates |
| Reminder | Surface actionable events | Lemon/Pomegranate attention bands | “Available” is not external delivery |
| Settings/Data control | Communicate privacy and consequences | Calm neutral surfaces; danger isolated | Delete/provider disclosure cannot be playful or ambiguous; export stays absent until R2 |

## 8. Interaction and motion

- Touch has no hover dependency. Every action remains obvious without motion.
- Primary actions move at most 1–2px on press; they do not rotate or leave their
  hit area. Destructive and confirmation controls never use playful motion.
- Pointer-only secondary controls may use a reduced 1–2° rotation and 2–4px hard
  shadow if it does not cause layout shift.
- Respect `prefers-reduced-motion`; all functional state changes remain visible
  with motion disabled.
- Async recognition, saving and deletion use state text/progress, not indefinite
  decorative animation.
- Bottom navigation preserves position and labels; it does not bounce or reorder.

## 9. Responsive and accessibility contract

- 320px width and 200% text must preserve all core actions without horizontal
  main-flow scroll.
- Primary touch targets are at least 44×44 CSS px; exceptions follow the PRD
  24px minimum plus spacing rule and require review.
- Focus order follows the visual/task order; focus is never removed for style.
- Dialogs trap focus, expose a name/description, and return focus on close.
- Errors attach to fields and appear in an actionable summary when necessary.
- Dynamic task status uses appropriate live regions without repeating noisy
  updates.
- Product/evidence images have purpose-based alt text without leaking OCR or
  personal content.
- Contrast is verified from actual rendered tokens, including muted text and
  colored sections.

## 10. Design deliverables before page code

1. Token sheet covering light theme, semantic states, typography, spacing,
   radius, shadow, focus and motion.
2. Responsive app shell and navigation at 320/390/768/1024+ widths.
3. Shared component state board for section 6.
4. Record/Today, Add/Capture/Confirm, Cabinet/Product and Plan reference screens.
5. Empty/loading/error/conflict/degraded/deleting states for each page group.
6. Keyboard, reduced-motion and 200% zoom annotations.
7. Comparison board: user `DESIGN.md` direction, MVP/Web interaction reference,
   current Uni implementation, proposed R1 screen and intentional differences.

No visual implementation is accepted solely because it resembles Clay. It must
also pass the PRD information hierarchy, business-state, responsive, browser and
accessibility gates.
