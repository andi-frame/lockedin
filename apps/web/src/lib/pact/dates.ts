// Calendar dates here are plain "YYYY-MM-DD" strings, the same shape as the API's `date` fields.
// Nothing in this file uses the browser's own zone: a pact's dates belong to the pact's timezone
// (AGENTS invariant 4), so "today" is always asked for a named zone and a given instant.

const shape = /^(\d{4})-(\d{2})-(\d{2})$/;
const DAY_MS = 86_400_000;

function parse(iso: string): number {
  const m = shape.exec(iso);
  if (!m) throw new RangeError(`not a date: ${iso}`);
  return Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

const format = (ms: number) => new Date(ms).toISOString().slice(0, 10);

export function isDate(raw: string): boolean {
  const m = shape.exec(raw);
  if (!m) return false;
  // Date.UTC rolls an impossible day (02-30) into the next month, so a round trip catches it.
  return format(parse(raw)) === raw;
}

export function addDays(iso: string, n: number): string {
  return format(parse(iso) + n * DAY_MS);
}

/** Signed number of days from `from` to `to`. */
export function daysBetween(from: string, to: string): number {
  return Math.round((parse(to) - parse(from)) / DAY_MS);
}

/** ISO weekday: Monday is 1, Sunday is 7 (SPEC §4 `schedule`). */
export function isoWeekday(iso: string): number {
  const d = new Date(parse(iso)).getUTCDay();
  return d === 0 ? 7 : d;
}

export function isTimeZone(zone: string): boolean {
  if (!zone) return false;
  try {
    new Intl.DateTimeFormat("en", { timeZone: zone });
    return true;
  } catch {
    return false;
  }
}

/** The calendar date it is in `zone` at `now`. */
export function todayIn(zone: string, now: Date): string {
  // en-CA formats as YYYY-MM-DD.
  return new Intl.DateTimeFormat("en-CA", { timeZone: zone, year: "numeric", month: "2-digit", day: "2-digit" }).format(now);
}
