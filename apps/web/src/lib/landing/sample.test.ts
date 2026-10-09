import { expect, test } from "bun:test";
import { SAMPLE, sampleLines } from "./sample";

// The landing page shows a month of a pact, drawn as the product draws it, with example data. The
// example must obey the product's own rule: the saldo is the sum of the lines above it.
const days = SAMPLE.days;
const mark = (day: number, who: "doer" | "backer") => days.find((d) => d.day === day)?.[who];

test("the saldo of every line is the previous saldo plus its amount", () => {
  const lines = sampleLines();
  let saldo = 0;
  for (const l of lines) {
    saldo += l.amount;
    expect(l.saldo).toBe(saldo);
  }
  expect(lines[0]?.kind).toBe("pot_initial");
});

test("every missed day before today is exactly one printed line, and nothing else prints", () => {
  const lines = sampleLines();
  const missed = days.flatMap((d) => (["doer", "backer"] as const).filter((who) => d[who] === "missed").map((who) => ({ day: d.day, who })));
  expect(lines.filter((l) => l.kind !== "pot_initial")).toHaveLength(missed.length);
  for (const m of missed) {
    const kind = m.who === "doer" ? "doer_miss" : "backer_miss";
    expect(lines.filter((l) => l.kind === kind && l.day === m.day)).toHaveLength(1);
  }
});

test("a doer's miss takes coins out and a backer's miss puts coins in", () => {
  for (const l of sampleLines()) {
    if (l.kind === "doer_miss") expect(l.amount).toBeLessThan(0);
    if (l.kind === "backer_miss") expect(l.amount).toBeGreaterThan(0);
  }
});

test("exactly one day is today, nothing after it has happened, and everything before it has", () => {
  expect(days.filter((d) => d.today)).toHaveLength(1);
  const today = SAMPLE.today;
  for (const d of days) {
    for (const who of ["doer", "backer"] as const) {
      const m = d[who];
      if (d.day > today) expect(m === "future" || m === "none").toBe(true);
      if (d.day < today) expect(["approved", "missed", "rest", "none"]).toContain(m);
    }
  }
});

test("the month is laid out the way October 2026 falls (the 1st is a Thursday) and has 31 days", () => {
  expect(SAMPLE.firstWeekday).toBe(3); // Monday = 0
  expect(days).toHaveLength(31);
  expect(days[0]?.day).toBe(1);
  expect(days.at(-1)?.day).toBe(31);
});

test("the doer's days are Monday to Saturday and the backer's Monday to Friday, as the sample pact says", () => {
  for (const d of days) {
    const weekday = (SAMPLE.firstWeekday + d.day - 1) % 7;
    if (weekday === 6) expect(d.doer).toBe("none");
    if (weekday >= 5) expect(d.backer).toBe("none");
  }
  expect(mark(4, "doer")).toBe("none"); // a Sunday
});

test("the final saldo is the pot minus the doer's misses plus the backer's", () => {
  const doer = days.filter((d) => d.doer === "missed").length;
  const backer = days.filter((d) => d.backer === "missed").length;
  expect(sampleLines().at(-1)?.saldo).toBe(SAMPLE.pot - doer * SAMPLE.doerPenalty + backer * SAMPLE.backerPenalty);
});
