# ADR-0007: Review power model: auto-approve, dispute, backer override

Status: Accepted (2026-10-07)

## Context
The backer reviews the doer's proof but gains money by rejecting it. The product owner wants the backer to keep **more power**, while the doer stays protected.

## Decision
**Doer protections:**
- If the reviewer doesn't act within `review_window_hours`, the proof is auto-approved.
- A rejection must include a reason.
- The doer can dispute a rejection within `dispute_window_hours`. If the dispute is still unresolved after `dispute_resolution_hours`, it is decided in the doer's favor.

**Backer power:**
- The backer can override an auto-approval within `override_window_hours`, at most `max_overrides` times per pact. An override cannot be disputed.
- The backer resolves disputes on check-ins they reviewed.

**Rules for both:**
- Every use of power is logged in `decisions` and shown to both members.
- All of these numbers are part of the terms both members sign at the start.

## Consequences
- The backer can't quietly drain the pot: every use of power is bounded, counted, and visible.
- Review power follows the reviewer role on each check-in. If the backer also commits to a habit, the doer reviews the backer's proof. The override power always stays with the backer.
