import { describe, expect, test } from "bun:test";
import type { components } from "./api/schema";
import { dayLabel, groupToday, ledgerEntry, nextDeadline, restLeft } from "./today";

type Today = components["schemas"]["Today"];
type TodayCheckIn = components["schemas"]["TodayCheckIn"];
type TodayPact = components["schemas"]["TodayPact"];
type LedgerLine = components["schemas"]["LedgerLine"];
type Pact = components["schemas"]["Pact"];

const ME = "me";
const ref = { user_id: "p", display_name: "Andi", line_color: "#9D174D" };

const line = (over: Partial<LedgerLine>): LedgerLine => ({
  id: 1,
  kind: "pot_initial",
  amount: 1000,
  balance_after: 1000,
  created_at: "2026-10-08T00:00:00Z",
  ...over,
});

describe("ledgerEntry", () => {
  test("the starting pot is a credit with its own label", () => {
    expect(ledgerEntry(line({}))).toEqual({ side: "credit", coins: 1000, label: "Ledger.potInitial", params: {} });
  });

  test("a doer's miss is a debit that names the day", () => {
    expect(ledgerEntry(line({ kind: "doer_miss", amount: -50, check_in_local_date: "2026-10-08" }))).toEqual({
      side: "debit",
      coins: 50,
      label: "Ledger.missed",
      params: { date: "2026-10-08" },
    });
  });

  test("a backer's miss adds to the pot, so it is a credit", () => {
    expect(ledgerEntry(line({ kind: "backer_miss", amount: 50, check_in_local_date: "2026-10-08" })).side).toBe("credit");
  });

  test("a clamped penalty moves nothing and says why", () => {
    const e = ledgerEntry(line({ kind: "doer_miss", amount: 0, note: "clamped", check_in_local_date: "2026-10-08" }));
    expect(e).toMatchObject({ side: "none", coins: 0, label: "Ledger.clamped" });
  });

  test("a reversal gives coins back, a payout takes the pot out", () => {
    expect(ledgerEntry(line({ kind: "reversal", amount: 50 }))).toMatchObject({ side: "credit", label: "Ledger.reversal" });
    expect(ledgerEntry(line({ kind: "payout", amount: -950 }))).toMatchObject({ side: "debit", coins: 950, label: "Ledger.payout" });
  });
});

describe("restLeft", () => {
  const pact = (rest_days: number, used: number): Pact =>
    ({
      terms: { members: { [ME]: { rest_days } } },
      members: [{ user_id: ME, rest_days_used: used }],
    }) as unknown as Pact;

  test("is what the terms allow minus what was used", () => expect(restLeft(pact(2, 1), ME)).toBe(1));
  test("never goes below zero", () => expect(restLeft(pact(1, 3), ME)).toBe(0));
  test("is zero for someone who is not a member", () => expect(restLeft(pact(2, 0), "stranger")).toBe(0));
});

const checkIn = (pact_id: string, status: string, deadline: string, date = "2026-10-08"): TodayCheckIn =>
  ({
    check_in: { id: `${pact_id}-${date}`, pact_id, status, local_date: date, cutoff_at: deadline, submit_deadline: deadline, member_id: ME },
    pact_title: pact_id,
    member: ref,
  }) as unknown as TodayCheckIn;

const todayPact = (pact_id: string): TodayPact =>
  ({ pact_id, title: pact_id, status: "active", my_role: "doer", balance: 0, partner: ref, recent_ledger: [] }) as unknown as TodayPact;

const today = (check_ins: TodayCheckIn[], pacts: TodayPact[]): Today =>
  ({ server_time: "2026-10-08T10:00:00Z", my_check_ins: check_ins, review_queue_count: 0, pacts }) as Today;

describe("groupToday", () => {
  test("keeps the API's pact order and puts each check-in under its pact, soonest first", () => {
    const t = today(
      [
        checkIn("b", "open", "2026-10-09T16:00:00Z"),
        checkIn("a", "open", "2026-10-08T16:00:00Z", "2026-10-08"),
        checkIn("a", "open", "2026-10-07T16:00:00Z", "2026-10-07"),
      ],
      [todayPact("a"), todayPact("b")],
    );
    const g = groupToday(t);
    expect(g.map((s) => s.pact.pact_id)).toEqual(["a", "b"]);
    expect(g[0]?.checkIns.map((c) => c.check_in.local_date)).toEqual(["2026-10-07", "2026-10-08"]);
  });

  test("a pact with nothing due today still has its section", () => {
    expect(groupToday(today([], [todayPact("a")]))[0]?.checkIns).toEqual([]);
  });

  test("a check-in whose pact is not listed is dropped rather than shown without context", () => {
    expect(groupToday(today([checkIn("ghost", "open", "2026-10-08T16:00:00Z")], [todayPact("a")]))[0]?.checkIns).toEqual([]);
  });
});

describe("nextDeadline", () => {
  test("is the soonest deadline among open check-ins, with its pact", () => {
    const t = today(
      [
        checkIn("a", "open", "2026-10-09T16:00:00Z"),
        checkIn("b", "open", "2026-10-08T16:00:00Z"),
        checkIn("c", "submitted", "2026-10-08T12:00:00Z"),
        checkIn("d", "approved", "2026-10-08T11:00:00Z"),
      ],
      [],
    );
    expect(nextDeadline(t)).toEqual({ until: "2026-10-08T16:00:00Z", pactTitle: "b" });
  });

  test("is null when nothing is waiting for the doer to submit", () => {
    expect(nextDeadline(today([checkIn("c", "submitted", "2026-10-08T12:00:00Z")], []))).toBeNull();
    expect(nextDeadline(today([], []))).toBeNull();
  });
});

describe("dayLabel", () => {
  test.each([
    ["2026-10-08", "today"],
    ["2026-10-07", "yesterday"],
    ["2026-10-01", "date"],
    ["2026-10-09", "date"],
  ])("%s relative to 2026-10-08 is %s", (date, kind) => expect(dayLabel(date, "2026-10-08")).toBe(kind as "today" | "yesterday" | "date"));
});
