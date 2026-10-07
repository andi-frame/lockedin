# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

User-pinned: Next.js frontend (Bun runtime), Go + Fiber backend API, PostgreSQL (own Docker image), Redis (cache + job queue), Garage (S3-compatible object storage). Everything runs under Docker for dev/staging/production, plus a no-Docker mode and a hybrid mode (infra in Docker, apps native). Remaining choices delegated; see `docs/ARCHITECTURE.md` and `docs/adr/`.

## Users

Peer pairs: university and high-school students who agree to keep each other disciplined in studying. Each person in a pact can be a **backer** (puts up a coin reward), a **doer** (commits to a daily study commitment), or both. The relationship is between equals: friends, classmates, or study buddies, not parent and child.

They use Tepati at two moments:
- **The doer, at the end of a study session** (often late evening, on a phone, on the way to a deadline): submitting proof before the daily cutoff.
- **The backer, between their own tasks**: reviewing proof in a short queue, approving or rejecting it with a reason.

## Product Purpose

Tepati turns a promise to study into a contract with a cost. A backer puts up a reward pot in coins (for example Rp1.000.000 = 1000 coins). Each day the doer misses, the pot loses an agreed amount. If the backer is also a doer and misses a day, the backer adds an agreed amount to the pot. When the pact ends, the backer owes the doer the remaining pot.

Success means the doer keeps their daily commitment for the whole pact, and both people trust the ledger at the end.

## Positioning

A two-way peer pact where the stakes move in **both** directions. The doer's misses shrink the pot, and the backer's own misses grow it. Every coin movement is an append-only ledger entry tied to a specific day and a specific proof decision. The backer reviews the proof, but auto-approval and disputes protect the doer from a backer who rejects proof just to keep their money.

## Operating Context

- Proof is rich text plus attachments: images, short video, links, files. It is written during or right after a study session.
- Days are cut off in the pact's timezone (default `Asia/Jakarta`). An agreed grace period follows the cutoff.
- Settlement runs automatically after each cutoff. Nobody clicks "deduct".
- **MVP money model: coins are an IOU ledger.** No real money passes through Tepati. The backer pays the doer outside the app (bank transfer or e-wallet) when the pact ends, and Tepati only records it. Holding funds would need Bank Indonesia licensing (PJP). Real payments are a post-MVP decision.

## Capabilities and Constraints

- Pact rules are agreed by **both** parties before the pact activates: schedule (which days count), daily requirement, penalty per missed day for each doer, cap/floor rules, review window, auto-approve window, rest-day allowance (break days), dispute policy, and timezone.
- Review: the backer approves or rejects. A rejection requires a reason. If the backer does nothing within the review window, the proof is **auto-approved**.
- **The backer has final power** (user decision): the backer may override an auto-approval or decide a dispute, but every override is logged with a reason and visible to both people. Rules agreed at the start can limit this (for example a maximum number of overrides per pact).
- Disputes: the doer can dispute a rejection. MVP resolution is backer-final with a mandatory written rationale. An optional neutral third-party referee is a post-MVP feature.
- Upload limits: uploads are compressed server-side (images to WebP or AVIF, video transcoded or capped) and rejected above a hard size limit.
- Terminology: *pact* (kontrak), *backer* (penyokong), *doer* (pelaku), *pot* (pot koin), *proof* (bukti), *check-in*, *cutoff*, *rest day* (hari jeda), *dispute* (sanggahan), *ledger* (buku koin).

## Brand Commitments

- Name: **Tepati** (from *tepati janji*, "keep your promise"). The repository folder is still `lockedin`.
- UI language: Bahasa Indonesia by default, i18n-ready for English.
- Voice: direct, peer-to-peer, honest about money, never preachy, and no hustle-culture hype.

## Evidence on Hand

None yet: no users, testimonials, metrics, or partners. Future work must not invent them. Demo content must be labelled synthetic.

## Product Principles

1. **The ledger is the truth.** Every coin movement is explainable from a dated entry and its cause.
2. **Rules are agreed first, then automatic.** Neither party negotiates in the moment. The pact decides.
3. **Power is visible.** The backer's extra power is real, but it is always logged and shown to the doer.
4. **Proof should be quick to give and quick to judge.** Submitting is a few taps. Reviewing is a glance.
5. **The deadline is the interface.** Time left until cutoff is always the most important fact on screen.

## Accessibility & Inclusion

WCAG 2.2 AA. The doer flow is mobile-first, including camera capture for proof. Reduced motion is respected. Coin amounts are never conveyed by colour alone.
