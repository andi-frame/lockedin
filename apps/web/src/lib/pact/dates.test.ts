import { describe, expect, test } from "bun:test";
import { addDays, clockIn, daysBetween, isDate, isoWeekday, isTimeZone, todayIn } from "./dates";

describe("isDate", () => {
  test.each([
    ["2026-11-01", true],
    ["2026-02-29", false], // not a leap year
    ["2028-02-29", true],
    ["2026-13-01", false],
    ["2026-1-1", false],
    ["", false],
    ["tomorrow", false],
  ])("%s is %p", (raw, ok) => expect(isDate(raw)).toBe(ok));
});

describe("addDays and daysBetween", () => {
  test("cross month and year ends", () => {
    expect(addDays("2026-11-30", 1)).toBe("2026-12-01");
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
    expect(addDays("2026-03-01", -1)).toBe("2026-02-28");
  });
  test("daysBetween is the signed gap", () => {
    expect(daysBetween("2026-11-01", "2026-11-30")).toBe(29);
    expect(daysBetween("2026-11-30", "2026-11-01")).toBe(-29);
    expect(daysBetween("2026-11-01", "2026-11-01")).toBe(0);
  });
});

describe("isoWeekday", () => {
  test("Monday is 1 and Sunday is 7", () => {
    expect(isoWeekday("2026-11-02")).toBe(1);
    expect(isoWeekday("2026-11-08")).toBe(7);
  });
});

describe("todayIn", () => {
  // 2026-10-31 20:00 UTC is already the next morning in Jakarta (UTC+7), still the evening in UTC.
  const instant = new Date("2026-10-31T20:00:00Z");
  test("is the calendar date in the pact's zone, not the browser's", () => {
    expect(todayIn("Asia/Jakarta", instant)).toBe("2026-11-01");
    expect(todayIn("UTC", instant)).toBe("2026-10-31");
    expect(todayIn("America/Los_Angeles", instant)).toBe("2026-10-31");
  });
});

describe("clockIn", () => {
  // The terms print 23:59, so every screen prints a time the same way, whatever the UI language.
  test("is HH:MM with a colon in the zone asked for", () => {
    expect(clockIn("2026-10-08T16:59:00Z", "Asia/Jakarta")).toBe("23:59");
    expect(clockIn("2026-10-08T17:29:00Z", "Asia/Jakarta")).toBe("00:29");
    expect(clockIn("2026-10-08T17:29:00Z", "UTC")).toBe("17:29");
  });
});

describe("isTimeZone", () => {
  test.each([
    ["Asia/Jakarta", true],
    ["UTC", true],
    ["Mars/Olympus", false],
    ["", false],
  ])("%s is %p", (zone, ok) => expect(isTimeZone(zone)).toBe(ok));
});
