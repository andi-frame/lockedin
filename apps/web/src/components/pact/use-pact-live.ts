"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "@/lib/api/browser";
import type { components } from "@/lib/api/schema";
import { unwrap } from "@/lib/api/unwrap";
import { arrivals, mergeLines, PAGE_SIZE } from "@/lib/passbook";

type LedgerLine = components["schemas"]["LedgerLine"];
type LedgerPage = components["schemas"]["LedgerPage"];
type CheckIn = components["schemas"]["CheckIn"];

/** How often the page asks again. A miss is printed by the worker, not by anything this tab did. */
export const POLL_MS = 30_000;

export type MoreState = "idle" | "loading" | "error";

/**
 * The pact page's live data: the passbook lines (first page from the server, older pages on demand)
 * and the calendar's check-ins. Every 30 s, and whenever the tab comes back, the first page is
 * fetched again; lines newer than anything shown are the "arrivals" the page prints with motion.
 * The balance and every `balance_after` come from the server; nothing is added up here.
 */
export function usePactLive(pactId: string, initial: { ledger: LedgerPage; checkIns: CheckIn[] }) {
  const [lines, setLines] = useState<LedgerLine[]>(initial.ledger.items);
  const [cursor, setCursor] = useState<string | null>(initial.ledger.next_cursor);
  const [balance, setBalance] = useState(initial.ledger.balance);
  const [checkIns, setCheckIns] = useState<CheckIn[]>(initial.checkIns);
  const [fresh, setFresh] = useState<ReadonlySet<number>>(() => new Set());
  const [lastPrinted, setLastPrinted] = useState<LedgerLine | null>(null);
  const [more, setMore] = useState<MoreState>("idle");

  // The refresh reads the lines on screen without being rebuilt every time they change.
  const shown = useRef(lines);
  useEffect(() => {
    shown.current = lines;
  }, [lines]);
  const moreRef = useRef<MoreState>("idle");

  const refresh = useCallback(async () => {
    try {
      const [page, list] = await Promise.all([
        unwrap(api.GET("/pacts/{pactId}/ledger", { params: { path: { pactId }, query: { limit: PAGE_SIZE } } })),
        unwrap(api.GET("/pacts/{pactId}/check-ins", { params: { path: { pactId } } })),
      ]);
      const printed = arrivals(shown.current, page.items);
      setLines((cur) => mergeLines(cur, page.items));
      setBalance(page.balance);
      setCheckIns(list.items);
      if (printed.length > 0) {
        setFresh((cur) => new Set([...cur, ...printed.map((l) => l.id)]));
        setLastPrinted(printed[0] ?? null);
      }
    } catch {
      // The last good page stays on screen; the next tick tries again.
    }
  }, [pactId]);

  useEffect(() => {
    const tick = () => {
      if (!document.hidden) void refresh();
    };
    const timer = window.setInterval(tick, POLL_MS);
    document.addEventListener("visibilitychange", tick);
    return () => {
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", tick);
    };
  }, [refresh]);

  const loadMore = useCallback(async () => {
    if (!cursor || moreRef.current === "loading") return;
    moreRef.current = "loading";
    setMore("loading");
    try {
      const page = await unwrap(api.GET("/pacts/{pactId}/ledger", { params: { path: { pactId }, query: { limit: PAGE_SIZE, cursor } } }));
      setLines((cur) => mergeLines(cur, page.items));
      setCursor(page.next_cursor);
      moreRef.current = "idle";
      setMore("idle");
    } catch {
      moreRef.current = "error";
      setMore("error");
    }
  }, [cursor, pactId]);

  return { lines, balance, checkIns, fresh, lastPrinted, hasMore: cursor !== null, more, loadMore, refresh };
}
