// Money is int64 coins and never a float (AGENTS invariant 5). Display goes through Intl with the
// Indonesian grouping (1.000), and the Rupiah equivalent is computed with BigInt so a large pot
// times the rate cannot lose precision.
export type Direction = "debit" | "credit" | "balance";

const grouping = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 0, useGrouping: true });

function assertCoins(n: number): void {
  if (!Number.isSafeInteger(n)) throw new RangeError(`coins must be a safe integer, got ${n}`);
}

/** Digits of the absolute amount. The sign comes from the direction, not from the number. */
export function formatCoins(n: number): string {
  assertCoins(n);
  return grouping.format(Math.abs(n));
}

export function coinsToIdr(coins: number, ratePerCoin: number): bigint {
  assertCoins(coins);
  if (!Number.isSafeInteger(ratePerCoin) || ratePerCoin < 1) throw new RangeError("rate must be a positive integer");
  return BigInt(Math.abs(coins)) * BigInt(ratePerCoin);
}

/** "Rp1.000.000", with no space, as ARCHITECTURE §7 shows it. */
export function formatIdr(coins: number, ratePerCoin: number): string {
  return `Rp${grouping.format(coinsToIdr(coins, ratePerCoin))}`;
}

/** A true minus (U+2212) lines up with the plus in tabular figures, a hyphen does not. */
export function signSymbol(direction: Direction): string {
  return direction === "debit" ? "−" : direction === "credit" ? "+" : "";
}
