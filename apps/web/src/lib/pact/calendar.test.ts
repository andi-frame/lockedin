import { describe, expect, test } from "bun:test";
import type { components } from "../api/schema";
import { dayCells, monthOf, monthsBetween, shiftMonth, startMonth, weeksOf } from "./calendar";

type CheckIn = components["schemas"]["CheckIn"];

const ci = (member: string, date: string, status: CheckIn["status"]): CheckIn => ({
  id: `${member}-${date}`,
  pact_id: "p",
  member_id: member,
  reviewer_id: "x",
  local_date: date,
  status,
  is_final: false,
  cutoff_at: "2026-10-08T16:59:00Z",
  submit_deadline: "2026-10-08T16:59:00Z",
  penalty_applied: false,
});

describe("months", () => {
  test("monthOf cuts a date to its month", () => {
    expect(monthOf("2026-10-08")).toBe("2026-10");
  });

  test("monthsBetween lists every month the pact touches, ends included", () => {
    expect(monthsBetween("2026-09-28", "2026-11-02")).toEqual(["2026-09", "2026-10", "2026-11"]);
    expect(monthsBetween("2026-10-01", "2026-10-31")).toEqual(["2026-10"]);
    expect(monthsBetween("2026-12-20", "2027-01-05")).toEqual(["2026-12", "2027-01"]);
  });

  test("shiftMonth crosses a year", () => {
    expect(shiftMonth("2026-12", 1)).toBe("2027-01");
    expect(shiftMonth("2027-01", -1)).toBe("2026-12");
    expect(shiftMonth("2026-10", 0)).toBe("2026-10");
  });

  test("startMonth opens on today's month, held inside the pact", () => {
    expect(startMonth("2026-10-08", "2026-09-01", "2026-11-30")).toBe("2026-10");
    expect(startMonth("2026-08-01", "2026-09-01", "2026-11-30")).toBe("2026-09"); // pact not started
    expect(startMonth("2027-01-01", "2026-09-01", "2026-11-30")).toBe("2026-11"); // pact over
  });
});

describe("weeksOf", () => {
  test("a week starts on Monday and rows are always seven days", () => {
    const weeks = weeksOf("2026-10");
    expect(weeks.every((w) => w.length === 7)).toBe(true);
    expect(weeks[0]?.[0]).toBe("2026-09-28"); // 1 Oct 2026 is a Thursday
    expect(weeks[0]?.[3]).toBe("2026-10-01");
    expect(weeks.at(-1)?.[6]).toBe("2026-11-01");
  });

  test("a month that starts on Monday has no leading days", () => {
    expect(weeksOf("2026-06")[0]?.[0]).toBe("2026-06-01");
  });

  test("February 2027 fits four rows exactly", () => {
    const weeks = weeksOf("2027-02");
    expect(weeks).toHaveLength(4);
    expect(weeks[0]?.[0]).toBe("2027-02-01");
    expect(weeks[3]?.[6]).toBe("2027-02-28");
  });
});

describe("dayCells", () => {
  const members = ["a", "b"];
  const items = [ci("a", "2026-10-07", "approved"), ci("b", "2026-10-07", "missed"), ci("a", "2026-10-08", "open")];

  test("a day inside the pact lists each member's status in member order", () => {
    const cells = dayCells(["2026-10-07", "2026-10-08"], items, members, { startsOn: "2026-10-01", endsOn: "2026-10-31", today: "2026-10-08", month: "2026-10" });
    expect(cells[0]).toMatchObject({ date: "2026-10-07", inPact: true, today: false, outside: false });
    expect(cells[0]?.marks.map((m) => [m.memberId, m.status])).toEqual([["a", "approved"], ["b", "missed"]]);
    expect(cells[1]).toMatchObject({ today: true });
    expect(cells[1]?.marks.map((m) => m.memberId)).toEqual(["a"]); // b has no check-in that day
  });

  test("days of the neighbouring month are outside and days beyond the pact are not in it", () => {
    const cells = dayCells(["2026-09-30", "2026-10-31", "2026-11-01"], [], members, { startsOn: "2026-10-05", endsOn: "2026-10-20", today: "2026-10-08", month: "2026-10" });
    expect(cells.map((c) => [c.outside, c.inPact])).toEqual([[true, false], [false, false], [true, false]]);
  });
});
