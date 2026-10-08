import type { components } from "../api/schema";
import { addDays, isoWeekday } from "./dates";

type CheckIn = components["schemas"]["CheckIn"];

// A month is "YYYY-MM". Weeks start on Monday, like the schedule's ISO weekdays (SPEC §4).

export const monthOf = (date: string): string => date.slice(0, 7);

export function shiftMonth(month: string, by: number): string {
  const [y = 0, m = 1] = month.split("-").map(Number);
  const index = y * 12 + (m - 1) + by;
  const year = Math.floor(index / 12);
  return `${year}-${String((index % 12) + 1).padStart(2, "0")}`;
}

/** Every month from the one holding `from` to the one holding `to`, both included. */
export function monthsBetween(from: string, to: string): string[] {
  const out: string[] = [];
  for (let m = monthOf(from); m <= monthOf(to); m = shiftMonth(m, 1)) out.push(m);
  return out;
}

/** The month the calendar opens on: today's, held inside the pact's own months. */
export function startMonth(today: string, startsOn: string, endsOn: string): string {
  const m = monthOf(today);
  if (m < monthOf(startsOn)) return monthOf(startsOn);
  if (m > monthOf(endsOn)) return monthOf(endsOn);
  return m;
}

/** The month as rows of seven dates, Monday first, padded with the neighbouring months' days. */
export function weeksOf(month: string): string[][] {
  const first = `${month}-01`;
  const last = addDays(`${shiftMonth(month, 1)}-01`, -1);
  const from = addDays(first, -(isoWeekday(first) - 1));
  const to = addDays(last, 7 - isoWeekday(last));
  const weeks: string[][] = [];
  for (let d = from; d <= to; d = addDays(d, 7)) {
    weeks.push(Array.from({ length: 7 }, (_, i) => addDays(d, i)));
  }
  return weeks;
}

export type DayMark = { memberId: string; status: CheckIn["status"]; checkIn: CheckIn };

export type DayCell = {
  date: string;
  /** Between the pact's first and last day. */
  inPact: boolean;
  today: boolean;
  /** Belongs to the previous or next month and is only there to fill the row. */
  outside: boolean;
  /** One mark per member who has a check-in that day, in `memberIds` order. */
  marks: DayMark[];
};

export function dayCells(
  dates: string[],
  checkIns: CheckIn[],
  memberIds: string[],
  ctx: { startsOn: string; endsOn: string; today: string; month: string },
): DayCell[] {
  const key = (date: string, member: string) => `${date}|${member}`;
  const byDay = new Map(checkIns.map((c) => [key(c.local_date, c.member_id), c]));
  return dates.map((date) => ({
    date,
    inPact: date >= ctx.startsOn && date <= ctx.endsOn,
    today: date === ctx.today,
    outside: monthOf(date) !== ctx.month,
    marks: memberIds.flatMap((memberId) => {
      const checkIn = byDay.get(key(date, memberId));
      return checkIn ? [{ memberId, status: checkIn.status, checkIn }] : [];
    }),
  }));
}
