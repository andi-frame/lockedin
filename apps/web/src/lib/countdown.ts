// The server decides deadlines (AGENTS invariant 4); the browser only counts down to a given instant.
export type Remaining = { totalSeconds: number; hours: number; minutes: number; seconds: number; expired: boolean };

export function remaining(untilMs: number, nowMs: number): Remaining {
  // Round up: until the deadline has truly passed the clock must not read 00:00:00.
  const totalSeconds = Math.max(0, Math.ceil((untilMs - nowMs) / 1000));
  return {
    totalSeconds,
    hours: Math.floor(totalSeconds / 3600),
    minutes: Math.floor((totalSeconds % 3600) / 60),
    seconds: totalSeconds % 60,
    expired: totalSeconds === 0,
  };
}

const pad = (n: number) => String(n).padStart(2, "0");

export function formatClock(r: Pick<Remaining, "hours" | "minutes" | "seconds">): string {
  return `${pad(r.hours)}:${pad(r.minutes)}:${pad(r.seconds)}`;
}

/**
 * Whole minutes left, rounded up, or null once expired. The visible digits tick every second, but
 * the screen-reader text is derived from this, so it changes once a minute and aria-live="polite"
 * stays usable (PLAN 5.2).
 */
export function spokenMinutes(r: Remaining): number | null {
  return r.expired ? null : Math.ceil(r.totalSeconds / 60);
}
