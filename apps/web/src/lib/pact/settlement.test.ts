import { describe, expect, test } from "bun:test";
import type { components } from "../api/schema";
import { noteState, settlementState } from "./settlement";

type Payout = components["schemas"]["Payout"];

const payout = (over: Partial<Payout> = {}): Payout => ({ amount: 900, ...over });

describe("settlementState", () => {
  test("while settling, the backer can mark it paid once, and the doer can confirm", () => {
    const backer = settlementState({ status: "settling", my_role: "backer", payout: payout() });
    expect(backer).toMatchObject({ phase: "settling", amount: 900, canMarkPaid: true, canConfirm: false, markedPaid: false });
    const doer = settlementState({ status: "settling", my_role: "doer", payout: payout() });
    expect(doer).toMatchObject({ canMarkPaid: false, canConfirm: true });
  });

  test("once the backer marked it paid, there is nothing more for the backer to do, and the doer still confirms", () => {
    const paid = payout({ marked_paid_at: "2026-10-08T10:00:00Z", marked_paid_note: "Transfer BCA" });
    expect(settlementState({ status: "settling", my_role: "backer", payout: paid })).toMatchObject({ canMarkPaid: false, markedPaid: true, note: "Transfer BCA" });
    expect(settlementState({ status: "settling", my_role: "doer", payout: paid })?.canConfirm).toBe(true);
  });

  test("the doer's confirmation alone completes it, so the doer can confirm before any mark", () => {
    expect(settlementState({ status: "settling", my_role: "doer", payout: payout() })?.canConfirm).toBe(true);
  });

  test("a completed pact is a summary: no actions for anyone, and who did what", () => {
    const done = payout({ marked_paid_at: "2026-10-08T10:00:00Z", confirmed_at: "2026-10-08T11:00:00Z" });
    for (const role of ["backer", "doer"] as const) {
      expect(settlementState({ status: "completed", my_role: role, payout: done })).toMatchObject({
        phase: "completed",
        canMarkPaid: false,
        canConfirm: false,
        markedPaid: true,
        confirmed: true,
      });
    }
    expect(settlementState({ status: "completed", my_role: "doer", payout: payout({ confirmed_at: "2026-10-08T11:00:00Z" }) })?.markedPaid).toBe(false);
  });

  test("a pot that ran out owes nothing, but is still confirmed", () => {
    const s = settlementState({ status: "settling", my_role: "doer", payout: payout({ amount: 0 }) });
    expect(s).toMatchObject({ owes: false, canConfirm: true });
    expect(settlementState({ status: "settling", my_role: "doer", payout: payout() })?.owes).toBe(true);
  });

  test("is null before the pact settles, or when no payout is attached", () => {
    expect(settlementState({ status: "active", my_role: "backer", payout: null })).toBeNull();
    expect(settlementState({ status: "settling", my_role: "backer", payout: null })).toBeNull();
    expect(settlementState({ status: "scheduled", my_role: "doer" })).toBeNull();
  });
});

describe("noteState", () => {
  test("an empty note is no note, and the text sent is trimmed", () => {
    expect(noteState("   ")).toEqual({ ok: true, value: null, length: 0 });
    expect(noteState("  Transfer BCA ")).toEqual({ ok: true, value: "Transfer BCA", length: 12 });
  });

  test("allows 500 characters and no more, counted as characters", () => {
    expect(noteState("a".repeat(500)).ok).toBe(true);
    expect(noteState("a".repeat(501)).ok).toBe(false);
    expect(noteState("😀".repeat(500)).ok).toBe(true);
  });
});
