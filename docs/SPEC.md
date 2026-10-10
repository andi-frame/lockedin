# Tepati: Functional Specification (MVP)

> Status: approved direction, pre-implementation. Product truth lives in `/PRODUCT.md`. This file is the **behavioural contract**: if code and this file disagree, either the code is wrong or this file must be updated in the same PR.

## 1. Glossary

| Term (EN, code) | UI (ID) | Meaning |
|---|---|---|
| pact | kontrak | An agreement between two people with a start date, an end date, and rules. |
| backer | penyokong | The member who funds the pot. Exactly one per pact in the MVP. |
| doer | pelaku | A member who has a daily commitment. The non-backer is always a doer; the backer may also be one (`backer_commits = true`). |
| pot | pot koin | Coins the backer will owe the doer at the end. |
| coin | koin | Integer unit. The rate is fixed per pact (default 1 coin = Rp1.000). |
| check-in | setoran harian | One row per (pact, doer, scheduled local date). |
| proof | bukti | Rich-text body plus attachments submitted for a check-in. |
| cutoff | batas waktu | The local time on a date after which a check-in can no longer be submitted (after grace). |
| grace | masa tenggang | Minutes after cutoff during which submission is still accepted. |
| review window | jendela tinjauan | How long the reviewer has to decide before auto-approval. |
| override | pembatalan | The backer reversing an auto-approval to a rejection. Limited and logged. |
| dispute | sanggahan | A doer contesting a rejection. |
| rest day | hari jeda | A day declared in advance by a doer; it carries no penalty. Limited per pact. |
| ledger entry | baris buku | An immutable signed coin movement. |
| terms | ketentuan | A frozen JSON snapshot of all rules, accepted by both members via their hash. |

## 2. Actors and roles

- **Member A (backer)**: creates the pact (in the MVP the creator is always the backer; the doer joins through the invite link, which re-keys the doer slot in the terms and clears any signature), funds the pot, and reviews B's proof. If `backer_commits` is set, A is also a doer, and B reviews A's proof.
- **Member B (doer)**: commits daily and reviews A's proof when A also commits.
- **System**: the settlement worker. It is the only actor that applies time-based transitions.
- **Reviewer of a check-in**: always *the other member*. Nobody reviews their own check-ins.

### Backer power (user decision: "A has more power")

The backer's extra powers apply **only to check-ins the backer reviews**, which are the doer's. They never apply to the backer's own check-ins.

1. **Override**: turn an `auto_approved` check-in into `rejected` within `override_window_hours` (default 48), at most `max_overrides` times per pact (default 3, and `0` disables it). A reason of at least 10 characters is required. The doer **cannot** dispute an override. That is the power.
2. **Dispute resolution**: the backer resolves disputes on check-ins they reviewed, with a mandatory rationale.
3. Every use of power writes a `decisions` row and is visible to both members on the check-in and in the pact activity feed. Power is real but never silent.

## 3. Pact lifecycle

```
draft ──propose──▶ proposed ──both accept same terms_hash──▶ scheduled ──starts_on reached──▶ active ──ends_on settled──▶ settling ──payout confirmed──▶ completed
  │                    │  ▲                                     │
  │                    │  └──edit terms (resets acceptances)───┘ (before start only)
  └──delete            └──decline/cancel──▶ cancelled
```

- `draft`: the creator edits freely and the invitee cannot see it yet.
- `proposed`: an invite link or email has been sent. Either party can edit the terms. Any edit creates a new `terms_version`, recomputes `terms_hash`, and clears both acceptances.
- `scheduled`: both members accepted the same `terms_hash`. The terms are now frozen. The pot is created with a `pot_initial` ledger entry. Check-in rows are generated for every scheduled date and doer.
- `active`: on or after `starts_on` in the pact timezone.
- `settling`: every check-in up to `ends_on` has reached a final status and the final balance is known. A `payout` record is created.
- `completed`: the backer marked the payout paid **and** the doer confirmed receipt, or the doer confirmed alone (the doer's confirmation is sufficient).
- `cancelled`: only before `scheduled`, or by mutual agreement while active (post-MVP; MVP allows cancel only before `scheduled`).

Constraints: `ends_on - starts_on` lies between 1 and 366 days. A user can be in at most 10 non-terminal pacts. A pact cannot be created, edited, proposed or signed once `starts_on` is before today in the pact timezone (`pact.start_passed`, 409): scheduling it would generate check-ins that are overdue at once and move coins for days nobody could have met. The start day itself is still allowed, since its cutoff is ahead. The way out is to move the dates later, which clears the signatures like every edit (decided 2026-10-09; the most conservative reading, the one that moves fewer coins).

## 4. Terms (agreed upfront by both)

Stored as `pacts.terms` (JSONB, validated) plus `terms_hash = sha256(canonical_json(terms))`.

```jsonc
{
  "version": 1,
  "timezone": "Asia/Jakarta",
  "starts_on": "2026-11-01",
  "ends_on": "2026-11-30",
  "cutoff_local_time": "23:59",
  "grace_minutes": 30,
  "coin_rate_idr": 1000,                 // 1 coin = Rp1.000 (display only; IOU)
  "initial_pot": 1000,
  "pot_floor": 0,                         // pot never drops below this
  "pot_cap": 2000,                        // null = no cap; backer bonuses stop at cap
  "review_window_hours": 24,
  "dispute_window_hours": 24,
  "dispute_resolution_hours": 48,
  "override_window_hours": 48,
  "max_overrides": 3,
  "backer_commits": true,
  "members": {
    "<backer_user_id>": {
      "role": "backer",
      "commitment": "Belajar Kalkulus minimal 2 jam",   // shown daily
      "schedule": [1,2,3,4,5],          // ISO weekdays; empty = backer does not commit
      "penalty_per_miss": 50,           // coins ADDED to pot when backer misses
      "rest_days": 2,
      "evidence": { "min_attachments": 1, "min_words": 0 }
    },
    "<doer_user_id>": {
      "role": "doer",
      "commitment": "Latihan soal UTBK 50 soal",
      "schedule": [1,2,3,4,5,6],
      "penalty_per_miss": 50,           // coins DEDUCTED from pot when doer misses
      "rest_days": 2,
      "evidence": { "min_attachments": 1, "min_words": 20 }
    }
  }
}
```

Validation rules:
- `penalty_per_miss` ≥ 1, `initial_pot` ≥ 1, and `pot_cap` (if set) ≥ `initial_pot`.
- `grace_minutes` ≤ 180, review/dispute windows are between 1 and 72 hours, and `max_overrides` ≤ 10.
- `timezone` is an IANA zone. `cutoff_local_time` is `HH:MM`.
- The UI must show a plain-language summary of the terms ("Kalau kamu melewatkan 1 hari, pot berkurang 50 koin (Rp50.000)"), and acceptance requires ticking it and typing your display name (a lightweight "signature").

## 5. Check-in state machine

One row per `(pact_id, member_id, local_date)` for each date in the member's `schedule` within `[starts_on, ends_on]`.

```
                  submit (≤ cutoff+grace)            approve
   ┌──────┐ ───────────────────────────▶ ┌───────────┐ ─────────▶ ┌──────────┐
   │ open │                              │ submitted │            │ approved │ (final)
   └──────┘ ◀── edit allowed while ───── └───────────┘ ─────────▶ ┌──────────────┐
     │   │      submitted & before cutoff+grace │   review deadline │ auto_approved │──override──▶ rejected* (final, no dispute)
     │   │                                      │      passed (sys)  └──────────────┘
     │   │ declare rest (before cutoff)        │ reject (reason)
     │   ▼                                     ▼
     │ ┌──────┐ (final)                 ┌──────────┐ dispute window passed (sys) ─▶ rejected (final)
     │ │ rest │                         │ rejected │
     │ └──────┘                         └──────────┘
     │ cutoff+grace passed, not submitted (sys)    │ doer disputes (≤ dispute window)
     ▼                                              ▼
   ┌────────┐ (final)                        ┌──────────┐ backer upholds ─▶ approved (final)
   │ missed │                                │ disputed │ backer dismisses ─▶ rejected (final)
   └────────┘                                └──────────┘ resolution deadline passed (sys) ─▶ approved (final, doer wins by default)
```

Definitions (all instants in UTC; `local_date` is a `date` in the pact timezone):
- `cutoff_at = local_date @ cutoff_local_time in tz`
- `submit_deadline = cutoff_at + grace_minutes`
- `review_deadline = max(submitted_at, cutoff_at) + review_window_hours`
  - The reviewer always gets the full window measured from cutoff, so early submitters do not shrink it.
- `dispute_deadline = rejected_at + dispute_window_hours`
- `resolution_deadline = disputed_at + dispute_resolution_hours`
- `override_deadline = auto_approved_at + override_window_hours`

Reasons: `reject`, `override`, and dispute `dismiss` each require a written reason of at least 10 characters (enforced by a DB check on `decisions`). A dispute itself also requires a reason.

**Final** statuses: `approved`, `rest`, `missed`, `rejected` (after the dispute window or dismissal, or after an override), and `auto_approved` after `override_deadline` (or immediately when `max_overrides` is exhausted or `0`).

A submission is valid only if it meets the terms `evidence` rules **and** every attachment has `status = ready` (uploads still processing block submission; the UI shows progress).

Rest days: the doer declares one before `cutoff_at` for an `open` check-in, up to `rest_days` per pact. Retroactive rest is not allowed.

### Late edits

- A doer may edit proof while `submitted` and before `submit_deadline`. Each edit creates a new `proofs.version`, and the reviewer sees the latest one plus a "diedit" marker.
- After a rejection, a resubmission is allowed if `now < submit_deadline` (the status goes back to `submitted`). Otherwise the doer can only dispute.

## 6. Coins and the ledger

**Invariant L1.** `pot_balance(pact) = SUM(ledger_entries.amount WHERE pact_id = …)`. There is no mutable balance column. A cached value in Redis is allowed, but it must be invalidated on every insert.

**Invariant L2.** Ledger rows are never updated or deleted. Corrections are new rows (`kind = reversal`) that reference `reverses_entry_id`.

**Invariant L3.** Each economic event has a unique `idempotency_key`, so re-running settlement can never double-charge.

| Event | Kind | Amount | Idempotency key |
|---|---|---|---|
| Pact scheduled | `pot_initial` | `+initial_pot` | `pact:{id}:initial` |
| Doer check-in becomes final `missed`/`rejected` | `doer_miss` | `-min(penalty, balance - pot_floor)` | `checkin:{id}:penalty` |
| Backer check-in becomes final `missed`/`rejected` | `backer_miss` | `+min(penalty, pot_cap - balance)` (or `+penalty` when there is no cap) | `checkin:{id}:penalty` |
| A penalised check-in later becomes `approved` (dispute upheld) | `reversal` | `-(original amount)` | `checkin:{id}:reversal` |
| Pact settled | `payout` | `-balance` (pot to 0, owed to doer) | `pact:{id}:payout` |

- The clamp is computed **inside the same transaction**, holding `SELECT … FROM pacts WHERE id = $1 FOR UPDATE`, so concurrent settlements cannot overshoot the floor or cap.
- If the clamp yields `0`, still insert the row with `amount = 0` and `note = 'clamped'`. The passbook shows it ("Pot sudah di batas minimum"), and the event stays explainable.
- Penalties apply when a status becomes **final**, not on the first rejection. A pending dispute does not move coins.

## 7. Settlement worker

Settlement is a periodic job `settlement:sweep` that runs every minute (asynq scheduler). Each run:

1. **Close submissions**: `open` check-ins with `submit_deadline < now()` become `missed`.
2. **Auto-approve**: `submitted` with `review_deadline < now()` becomes `auto_approved`, plus a `decisions` row with actor `system`.
3. **Finalise auto-approvals**: `auto_approved` with `override_deadline < now()` becomes final. Coins are untouched.
4. **Finalise rejections**: `rejected` (not disputed) with `dispute_deadline < now()` becomes final and the penalty is applied.
5. **Expire disputes**: `disputed` with `resolution_deadline < now()` becomes `approved`, plus a decision by `system`.
6. **Apply penalties** for each check-in that just became final `missed`/`rejected` (see §6).
7. **Close pacts**: an `active` pact whose check-ins are all final and where `now() > ends_on` (local) becomes `settling`. Insert the `payout` entry and create a `payouts` row.

Every transition happens in its own DB transaction using `UPDATE … WHERE id = $1 AND status = $expected` (optimistic) together with the ledger insert. Process at most 500 rows per step per run (batch), ordered by deadline. Each transition also enqueues a `notify:*` task. The job must be safe to run concurrently on multiple workers, which is guaranteed by the expected-status guard and the idempotency keys.

User-triggered transitions (approve, reject, dispute, resolve, override, rest) go through the API and use the same domain service functions as the worker. There is exactly one implementation of each transition.

## 8. Proof and attachments

- Body: Tiptap/ProseMirror JSON (`body_doc`) plus derived plain text (`body_text`) for word counts and search. The server validates the JSON against an allow-list of node and mark types: paragraph, heading(2–3), bulletList, orderedList, listItem, blockquote, codeBlock, hardBreak, horizontalRule, image (an attachment reference only, never an external src), link (http/https only), bold, italic, strike, code, and taskList/taskItem. Any other node is rejected.
- Links: plain URLs in the body or the `links[]` field. Store only the URL and title; do not fetch previews in the MVP.
- Attachments: images, video, and documents (PDF). See the upload pipeline in `docs/ARCHITECTURE.md §6`.

| Kind | Accepted MIME (sniffed by magic bytes, not trusted from client) | Raw hard limit | Stored output |
|---|---|---|---|
| image | jpeg, png, webp, heic/heif, avif, gif (first frame) | 15 MB, 40 MP | WebP q80, longest side ≤ 2048 px, EXIF stripped, plus a 480 px thumbnail |
| video | mp4, quicktime, webm | 200 MB, 180 s | H.264 MP4 720p, CRF 28, AAC 96k, faststart, plus a poster JPEG |
| file | pdf | 20 MB | stored as-is, first-page thumbnail optional (post-MVP) |

- Limits are configurable via env (`UPLOAD_*`). The client pre-checks them and pre-compresses images in the browser. The server is the authority.
- Per user: at most 10 attachments per proof, at most 1 GB of stored media per pact (configurable), and an upload-intent rate limit of 30/min.
- An attachment over a limit after processing (for example a video still above 50 MB after transcode) is `rejected` with a reason and deleted.

## 9. Notifications (MVP: in-app + email; web push in phase 5)

| Trigger | To | Channel |
|---|---|---|
| Invite received | invitee | email |
| Terms changed or accepted | other member | in-app + email |
| 3 h and 30 min before cutoff with check-in still `open` | doer | in-app (push later) |
| Proof submitted | reviewer | in-app + email digest |
| Review deadline in 2 h | reviewer | in-app |
| Rejected, overridden, or auto-approved | doer | in-app + email |
| Dispute opened | backer | in-app + email |
| Pact settled, payout due | both | in-app + email |

**Switching emails off (decided 2026-10-09).** A person can turn off the *email* for these kinds: terms changed, terms signed, proof submitted (the digest), rejected, overridden, and auto-approved. Their content is also in the app, and missing one costs nothing. The email always goes out for an invite, for a dispute opened against your review (the backer has a deadline to decide it, or the doer wins) and for a settled pact (a payout depends on it). The in-app notification is never switched off. The list is stored as the kinds that are off, so a kind added later is on for everyone. A switched-off kind is marked as handled when the worker reaches it, so switching it back on does not mail what was missed meanwhile.

**The account (`PATCH /me`).** A person can change their display name, language and time zone. The name is also the signature typed on terms: signatures already given keep the name that was typed then, and the next signature must match the current name. The time zone is the default for new pacts and the zone of Today's date and the inbox; a pact keeps its own zone.

**The password.** A person can change it from Settings by giving the current one. That ends every other session and keeps the one asking; a wrong current password changes nothing and counts toward a per-user limit of 10 attempts a minute, so a stolen session cannot be used to guess it. There is no password reset by e-mail and no way to change the e-mail address in the MVP (both need an e-mail verification flow).

## 10. Non-functional requirements

- p95 API latency below 150 ms for reads and below 300 ms for writes at 200 RPS on a single 2 vCPU API instance (excluding uploads).
- Settlement lag: a transition happens at most 2 minutes after its deadline.
- No data loss on worker crash: tasks are retried with backoff, and a periodic sweep re-derives the state anyway.
- Every money-moving path has unit tests **and** an integration test against real Postgres.
- Time logic is tested with an injected clock at DST-free and timezone-boundary cases (`Asia/Jakarta`, `Asia/Makassar`, `Asia/Jayapura`, `UTC`).

## 11. Out of scope for the MVP (do not build)

Real money or payment gateways, more than two members per pact, third-party referees, AI proof verification, public feeds, social features, native mobile apps, and terms amendments after scheduling.
