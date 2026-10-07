import { describe, expect, test } from "bun:test";
import { coinsToIdr, formatCoins, formatIdr, signSymbol } from "./format";

describe("formatCoins", () => {
  const cases: [number, string][] = [
    [0, "0"],
    [7, "7"],
    [999, "999"],
    [1000, "1.000"],
    [12500, "12.500"],
    [1234567, "1.234.567"],
    [-1000, "1.000"], // the sign is shown separately, from the direction
  ];
  for (const [n, want] of cases) {
    test(`${n} -> ${want}`, () => expect(formatCoins(n)).toBe(want));
  }

  test("rejects a fractional amount", () => expect(() => formatCoins(1.5)).toThrow(RangeError));
  test("rejects an amount beyond the safe integer range", () =>
    expect(() => formatCoins(Number.MAX_SAFE_INTEGER + 2)).toThrow(RangeError));
});

describe("coinsToIdr", () => {
  test("multiplies exactly", () => expect(coinsToIdr(1000, 1000)).toBe(1_000_000n));
  test("does not lose precision past 2^53", () =>
    expect(coinsToIdr(9_007_199_254_740_991, 1000)).toBe(9_007_199_254_740_991_000n));
  test("rejects a non-positive rate", () => expect(() => coinsToIdr(5, 0)).toThrow(RangeError));
});

describe("formatIdr", () => {
  test("1000 coins at Rp1.000 per coin", () => expect(formatIdr(1000, 1000)).toBe("Rp1.000.000"));
  test("small amounts", () => expect(formatIdr(3, 500)).toBe("Rp1.500"));
  test("zero", () => expect(formatIdr(0, 1000)).toBe("Rp0"));
});

describe("signSymbol", () => {
  test("debit is a true minus, not a hyphen", () => expect(signSymbol("debit")).toBe("−"));
  test("credit is a plus", () => expect(signSymbol("credit")).toBe("+"));
  test("a balance has no sign", () => expect(signSymbol("balance")).toBe(""));
});
