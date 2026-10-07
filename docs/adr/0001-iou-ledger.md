# ADR-0001: Coins are an IOU ledger; no money flows through Tepati (MVP)

Status: Accepted (2026-10-07)

## Context
The product lets a backer promise money (for example Rp1.000.000) to a doer. If Tepati held those funds, it would be operating stored value or escrow. In Indonesia that requires a Bank Indonesia payment service provider (PJP) licence. Payment gateways such as Midtrans and Xendit can collect payments, but they don't exempt us from those rules if we hold funds over time.

## Decision
Coins are bookkeeping units. Each pact has a fixed display rate (`coin_rate_idr`). Tepati records what the backer owes. At settlement, the backer pays outside the app and marks the payout as paid, and the doer confirms it.

## Consequences
- The MVP needs no KYC, no payment gateway, and no reconciliation work.
- Trust depends on the relationship between the two people. The product's value is a transparent ledger.
- `payouts` is a separate table, so a later integration with a licensed escrow partner can attach to it without rewriting the ledger.
