import type { components } from "./api/schema";

type LedgerLine = components["schemas"]["LedgerLine"];

/** Lines per page. Smaller than a month of misses, so the second page has something to scroll to. */
export const PAGE_SIZE = 20;

/**
 * The lines already shown plus a page just fetched, newest first, each line once. Line ids only grow
 * (a line is appended, never edited: AGENTS invariant 1), so the id is the order.
 */
export function mergeLines(shown: LedgerLine[], fetched: LedgerLine[]): LedgerLine[] {
  const byId = new Map<number, LedgerLine>();
  for (const l of [...shown, ...fetched]) byId.set(l.id, l);
  return [...byId.values()].sort((a, b) => b.id - a.id);
}

/**
 * Lines on a refreshed first page that were not on the page before: what just got printed. The very
 * first load shows everything and prints nothing, so it has no arrivals.
 */
export function arrivals(shown: LedgerLine[], firstPage: LedgerLine[]): LedgerLine[] {
  if (shown.length === 0) return [];
  const newest = Math.max(...shown.map((l) => l.id));
  return firstPage.filter((l) => l.id > newest).sort((a, b) => b.id - a.id);
}

/**
 * The saldo while it rolls from `from` to `to`: whole coins, easing out so it slows as it lands on
 * the figure the server printed (the balance is never computed here, only displayed on its way).
 */
export function rollValue(from: number, to: number, progress: number): number {
  const p = Math.min(1, Math.max(0, progress));
  const eased = 1 - (1 - p) ** 3;
  return Math.round(from + (to - from) * eased);
}
