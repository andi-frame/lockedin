// The example month on the landing page: a 28-day pact in October 2026, drawn the way the product
// draws it. It is made up (the page says so) but it obeys the product's rule: the saldo of a line is
// the previous saldo plus its amount, and a line exists exactly for each missed day.

export type Mark = "approved" | "missed" | "rest" | "submitted" | "open" | "future" | "none";
export type SampleDay = { day: number; today: boolean; doer: Mark; backer: Mark };
export type SampleLine = { day: number; kind: "pot_initial" | "doer_miss" | "backer_miss"; amount: number; saldo: number };

const POT = 1000;
const DOER_PENALTY = 50;
const BACKER_PENALTY = 20;
const TODAY = 16;
const PACT_ENDS = 28;
const FIRST_WEEKDAY = 3; // 1 October 2026 is a Thursday; Monday = 0

// What happened on the days before today. Anything not listed was approved.
const DOER_MISSED = new Set([3, 12]);
const DOER_REST = new Set([7]);
const BACKER_MISSED = new Set([6]);

const weekdayOf = (day: number) => (FIRST_WEEKDAY + day - 1) % 7;

function markFor(who: "doer" | "backer", day: number): Mark {
  const weekday = weekdayOf(day);
  const scheduled = who === "doer" ? weekday <= 5 : weekday <= 4; // the doer Monday to Saturday, the backer Monday to Friday
  if (!scheduled || day > PACT_ENDS) return "none";
  if (day > TODAY) return "future";
  if (day === TODAY) return who === "doer" ? "submitted" : "open";
  if (who === "doer") return DOER_MISSED.has(day) ? "missed" : DOER_REST.has(day) ? "rest" : "approved";
  return BACKER_MISSED.has(day) ? "missed" : "approved";
}

export const SAMPLE = {
  firstWeekday: FIRST_WEEKDAY,
  today: TODAY,
  pactEnds: PACT_ENDS,
  pot: POT,
  doerPenalty: DOER_PENALTY,
  backerPenalty: BACKER_PENALTY,
  days: Array.from({ length: 31 }, (_, i): SampleDay => {
    const day = i + 1;
    return { day, today: day === TODAY, doer: markFor("doer", day), backer: markFor("backer", day) };
  }),
};

/** The printed lines of the example, oldest first, each with the saldo after it. */
export function sampleLines(): SampleLine[] {
  const lines: SampleLine[] = [];
  let saldo = 0;
  const push = (day: number, kind: SampleLine["kind"], amount: number) => {
    saldo += amount;
    lines.push({ day, kind, amount, saldo });
  };
  push(1, "pot_initial", POT);
  for (const d of SAMPLE.days) {
    if (d.doer === "missed") push(d.day, "doer_miss", -DOER_PENALTY);
    if (d.backer === "missed") push(d.day, "backer_miss", BACKER_PENALTY);
  }
  return lines;
}
