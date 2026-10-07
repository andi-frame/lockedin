import { describe, expect, test } from "bun:test";
import { formatClock, remaining, spokenMinutes } from "./countdown";

const at = (s: string) => Date.parse(s);

describe("remaining", () => {
  test("splits into hours, minutes and seconds", () => {
    const r = remaining(at("2026-10-08T22:00:00Z"), at("2026-10-08T18:18:48Z"));
    expect(r).toEqual({ totalSeconds: 13272, hours: 3, minutes: 41, seconds: 12, expired: false });
  });

  test("rounds a partial second up, so 00:00:00 means the time is really over", () => {
    expect(remaining(1500, 0).totalSeconds).toBe(2);
    expect(remaining(1, 0)).toMatchObject({ totalSeconds: 1, expired: false });
  });

  test("is expired at and after the deadline, never negative", () => {
    expect(remaining(1000, 1000)).toEqual({ totalSeconds: 0, hours: 0, minutes: 0, seconds: 0, expired: true });
    expect(remaining(1000, 9000).expired).toBe(true);
    expect(remaining(1000, 9000).hours).toBe(0);
  });

  test("keeps counting hours past a day instead of wrapping", () => {
    expect(remaining(100 * 3600 * 1000, 0).hours).toBe(100);
  });
});

describe("formatClock", () => {
  test("pads each part to two digits", () =>
    expect(formatClock({ hours: 3, minutes: 4, seconds: 5 })).toBe("03:04:05"));
  test("lets hours grow beyond two digits", () =>
    expect(formatClock({ hours: 100, minutes: 0, seconds: 0 })).toBe("100:00:00"));
});

describe("spokenMinutes", () => {
  test("is the same within a minute", () => {
    expect(spokenMinutes(remaining(10_000_000, 0))).toBe(spokenMinutes(remaining(10_000_000, 30_000)));
  });
  test("changes when the minute changes", () => {
    expect(spokenMinutes(remaining(10_000_000, 0))).not.toBe(spokenMinutes(remaining(10_000_000, 61_000)));
  });
  test("counts a started minute as a minute left", () => expect(spokenMinutes(remaining(61_000, 0))).toBe(2));
  test("is null at expiry", () => expect(spokenMinutes(remaining(0, 5))).toBeNull());
});
