import type { components } from "./api/schema";
import { addDays } from "./pact/dates";

// The Today screen's rules: which check-ins sit under which pact, which deadline the band counts
// to, and how a ledger line reads in the mini passbook. The API already orders pacts by their
// nearest deadline, so nothing here re-sorts them.

type Today = components["schemas"]["Today"];
type TodayCheckIn = components["schemas"]["TodayCheckIn"];
type TodayPact = components["schemas"]["TodayPact"];
type LedgerLine = components["schemas"]["LedgerLine"];
type Pact = components["schemas"]["Pact"];

export type LedgerEntry = {
  /** `none` is a line that moved no coins (a penalty clamped at the floor or cap). */
  side: "debit" | "credit" | "none";
  coins: number;
  label: `Ledger.${string}`;
  params: Record<string, string>;
};

/** One passbook line: a side, the absolute figure, and the keterangan as a message key. */
export function ledgerEntry(line: LedgerLine): LedgerEntry {
  const side = line.amount < 0 ? "debit" : line.amount > 0 ? "credit" : "none";
  const coins = Math.abs(line.amount);
  const date = line.check_in_local_date ?? "";
  switch (line.kind) {
    case "pot_initial":
      return { side, coins, label: "Ledger.potInitial", params: {} };
    case "doer_miss":
    case "backer_miss":
      // A penalty that could not move (the pot was at its floor or cap) is still printed, with why.
      return line.note === "clamped"
        ? { side: "none", coins: 0, label: "Ledger.clamped", params: { date } }
        : { side, coins, label: "Ledger.missed", params: { date } };
    case "reversal":
      return { side, coins, label: "Ledger.reversal", params: { date } };
    case "payout":
      return { side, coins, label: "Ledger.payout", params: {} };
  }
}

/** Rest days the member can still declare in this pact. */
export function restLeft(pact: Pact, me: string): number {
  const allowed = pact.terms.members[me]?.rest_days;
  const used = pact.members.find((m) => m.user_id === me)?.rest_days_used;
  if (allowed === undefined || used === undefined) return 0;
  return Math.max(0, allowed - used);
}

export type Section = { pact: TodayPact; checkIns: TodayCheckIn[] };

/** Today's check-ins under their pact, in the API's pact order, soonest deadline first. */
export function groupToday(today: Today): Section[] {
  return today.pacts.map((pact) => ({
    pact,
    checkIns: today.my_check_ins
      .filter((c) => c.check_in.pact_id === pact.pact_id)
      .sort((a, b) => Date.parse(a.check_in.submit_deadline) - Date.parse(b.check_in.submit_deadline)),
  }));
}

/** What the header band counts down to: the soonest deadline of a check-in still waiting to be sent. */
export function nextDeadline(today: Today): { until: string; pactTitle: string } | null {
  let best: TodayCheckIn | undefined;
  for (const c of today.my_check_ins) {
    if (c.check_in.status !== "open") continue;
    if (!best || Date.parse(c.check_in.submit_deadline) < Date.parse(best.check_in.submit_deadline)) best = c;
  }
  return best ? { until: best.check_in.submit_deadline, pactTitle: best.pact_title } : null;
}

/** How to name a check-in's date next to today's date, both in the pact's zone. */
export function dayLabel(localDate: string, today: string): "today" | "yesterday" | "date" {
  if (localDate === today) return "today";
  if (localDate === addDays(today, -1)) return "yesterday";
  return "date";
}
