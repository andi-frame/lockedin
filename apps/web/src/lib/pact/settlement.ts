import type { components } from "../api/schema";

type Pact = components["schemas"]["Pact"];
type Payout = components["schemas"]["Payout"];

export const NOTE_MAX = 500;

export type Settlement = {
  phase: "settling" | "completed";
  /** Coins the backer owes the doer, fixed when the pact settled. */
  amount: number;
  /** False when the pot ran out: nothing to pay, but the doer still closes the pact. */
  owes: boolean;
  markedPaid: boolean;
  markedPaidAt: string | null;
  note: string | null;
  confirmed: boolean;
  confirmedAt: string | null;
  canMarkPaid: boolean;
  canConfirm: boolean;
};

/**
 * What the settlement panel shows and offers (SPEC §3). The backer says it was paid, once; the
 * doer's confirmation alone completes the pact, with or without that. The server enforces both
 * (`pact.invalid_state`, `pact.not_backer`, `pact.not_doer`); this only decides which buttons
 * to draw. The amount is the payout row's, never computed from the ledger here.
 */
export function settlementState(pact: Pick<Pact, "status" | "my_role"> & { payout?: Payout | null }): Settlement | null {
  const payout = pact.payout;
  if (!payout || (pact.status !== "settling" && pact.status !== "completed")) return null;
  const settling = pact.status === "settling";
  return {
    phase: pact.status === "settling" ? "settling" : "completed",
    amount: payout.amount,
    owes: payout.amount > 0,
    markedPaid: Boolean(payout.marked_paid_at),
    markedPaidAt: payout.marked_paid_at ?? null,
    note: payout.marked_paid_note ?? null,
    confirmed: Boolean(payout.confirmed_at),
    confirmedAt: payout.confirmed_at ?? null,
    canMarkPaid: settling && pact.my_role === "backer" && !payout.marked_paid_at,
    canConfirm: settling && pact.my_role === "doer",
  };
}

/** The optional note on "marked paid": empty is no note, and what is sent is trimmed. */
export function noteState(raw: string): { ok: boolean; value: string | null; length: number } {
  const trimmed = raw.trim();
  const length = Array.from(trimmed).length;
  return { ok: length <= NOTE_MAX, value: trimmed === "" ? null : trimmed, length };
}
