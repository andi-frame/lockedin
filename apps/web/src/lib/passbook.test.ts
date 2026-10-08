import { describe, expect, test } from "bun:test";
import type { components } from "./api/schema";
import { arrivals, mergeLines, PAGE_SIZE, rollValue } from "./passbook";

type LedgerLine = components["schemas"]["LedgerLine"];

const line = (id: number): LedgerLine => ({ id, kind: "doer_miss", amount: -50, balance_after: 1000 - id, created_at: "2026-10-08T00:00:00Z" });
const ids = (lines: LedgerLine[]) => lines.map((l) => l.id);

describe("mergeLines", () => {
  test("joins pages newest first and drops a line seen twice", () => {
    expect(ids(mergeLines([line(9), line(8), line(7)], [line(8), line(7), line(6)]))).toEqual([9, 8, 7, 6]);
  });

  test("puts a refreshed first page in front of the pages already loaded", () => {
    const loaded = [line(5), line(4), line(3), line(2), line(1)];
    expect(ids(mergeLines(loaded, [line(7), line(6), line(5)]))).toEqual([7, 6, 5, 4, 3, 2, 1]);
  });

  test("keeps what it had when the refresh brings nothing", () => {
    expect(ids(mergeLines([line(2), line(1)], []))).toEqual([2, 1]);
  });
});

describe("arrivals", () => {
  test("are the lines newer than anything shown", () => {
    expect(ids(arrivals([line(5), line(4)], [line(7), line(6), line(5), line(4)]))).toEqual([7, 6]);
  });

  test("are empty when nothing is newer", () => {
    expect(arrivals([line(5)], [line(5), line(4)])).toEqual([]);
  });

  test("never count the first load as arrivals", () => {
    expect(arrivals([], [line(3), line(2), line(1)])).toEqual([]);
  });
});

test("a page is smaller than the seeded history, so the second page is reachable", () => {
  expect(PAGE_SIZE).toBeLessThan(33);
});

describe("rollValue", () => {
  test("starts at the old saldo and lands exactly on the new one", () => {
    expect(rollValue(660, 610, 0)).toBe(660);
    expect(rollValue(660, 610, 1)).toBe(610);
  });

  test("only ever shows whole coins and never overshoots", () => {
    for (let p = 0; p <= 1; p += 0.05) {
      const v = rollValue(660, 610, p);
      expect(Number.isInteger(v)).toBe(true);
      expect(v).toBeGreaterThanOrEqual(610);
      expect(v).toBeLessThanOrEqual(660);
    }
  });

  test("moves fast first and slows into the landing (ease-out)", () => {
    expect(660 - rollValue(660, 610, 0.5)).toBeGreaterThan(25);
  });

  test("a progress outside 0..1 is held to the ends", () => {
    expect(rollValue(10, 20, -1)).toBe(10);
    expect(rollValue(10, 20, 2)).toBe(20);
  });
});
