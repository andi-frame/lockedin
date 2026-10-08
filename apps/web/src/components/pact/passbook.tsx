"use client";

import { CircleNotch } from "@phosphor-icons/react";
import { useFormatter, useTranslations } from "next-intl";
import { useEffect, useRef, type CSSProperties } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/cn";
import type { components } from "@/lib/api/schema";
import { formatCoins, signSymbol } from "@/lib/format";
import type { MemberSlot } from "@/lib/member";
import { ledgerEntry } from "@/lib/today";
import { RollingAmount } from "./rolling-amount";
import type { MoreState } from "./use-pact-live";

type LedgerLine = components["schemas"]["LedgerLine"];

export type PassbookMember = { name: string; slot: MemberSlot };

const underline: Record<MemberSlot, string> = { 0: "decoration-member-a", 1: "decoration-member-b" };

/**
 * The full coin book: date, keterangan, debit, kredit and saldo, newest line first. Every figure is
 * the server's (`balance_after` is a window sum over the whole ledger, so it is right on every
 * page). A line that just arrived prints in: it slides in, its digits type left to right, and the
 * big saldo above rolls to the new value.
 */
export function Passbook({
  title,
  rate,
  timeZone,
  lines,
  balance,
  fresh,
  lastPrinted,
  members,
  hasMore,
  more,
  onLoadMore,
}: {
  title: string;
  rate: number;
  timeZone: string;
  lines: LedgerLine[];
  balance: number;
  fresh: ReadonlySet<number>;
  lastPrinted: LedgerLine | null;
  members: Record<string, PassbookMember>;
  hasMore: boolean;
  more: MoreState;
  onLoadMore: () => void;
}) {
  const t = useTranslations("Passbook");
  const tl = useTranslations("Ledger");
  const f = useFormatter();
  const short = (iso: string) => f.dateTime(new Date(iso), { day: "numeric", month: "short", timeZone });
  const day = (d: string) => f.dateTime(new Date(`${d}T00:00:00Z`), { day: "numeric", month: "short", timeZone: "UTC" });
  const note = (line: LedgerLine) => {
    const e = ledgerEntry(line);
    return tl(e.label.replace("Ledger.", ""), { date: e.params.date ? day(e.params.date) : "" });
  };

  // Older pages load as the end of the book scrolls into view; the button below does the same for
  // a keyboard or a browser without observers.
  const end = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const node = end.current;
    if (!node || !hasMore || more !== "idle" || typeof IntersectionObserver === "undefined") return;
    const watch = new IntersectionObserver((entries) => entries.some((e) => e.isIntersecting) && onLoadMore(), { rootMargin: "320px" });
    watch.observe(node);
    return () => watch.disconnect();
  }, [hasMore, more, onLoadMore, lines.length]);

  return (
    <section aria-labelledby="coin-book" className="min-w-0">
      <h2 id="coin-book" className="text-lg font-semibold tracking-tight">
        {title}
      </h2>
      <p className="mt-3 text-sm text-muted">{t("saldo")}</p>
      <RollingAmount coins={balance} delayMs={320} direction="balance" rate={rate} size="xl" />

      {lines.length === 0 ? (
        <p className="mt-4 text-[15px] text-muted">{tl("empty")}</p>
      ) : (
        <table className="mt-5 w-full border-collapse text-sm" data-testid="passbook">
          <caption className="sr-only">{t("caption")}</caption>
          <thead>
            <tr className="border-b border-rule-strong text-left text-[13px] text-muted">
              <th scope="col" className="w-[3.25rem] py-2 pr-2 font-medium sm:w-20">
                {t("colDate")}
              </th>
              <th scope="col" className="py-2 pr-2 font-medium">
                {t("colNote")}
              </th>
              <th scope="col" className="w-[3.25rem] py-2 pr-2 text-right font-medium sm:w-24">
                <span aria-hidden className="sm:hidden">
                  {t("debitShort")}
                </span>
                <span className="sr-only sm:not-sr-only">{t("colDebit")}</span>
              </th>
              <th scope="col" className="w-[3.25rem] py-2 pr-2 text-right font-medium sm:w-24">
                <span aria-hidden className="sm:hidden">
                  {t("creditShort")}
                </span>
                <span className="sr-only sm:not-sr-only">{t("colCredit")}</span>
              </th>
              <th scope="col" className="w-[4rem] py-2 text-right font-medium sm:w-28">
                {t("colSaldo")}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-rule">
            {lines.map((line) => {
              const e = ledgerEntry(line);
              const who = line.member_id ? members[line.member_id] : undefined;
              const isNew = fresh.has(line.id);
              const typed = (text: string) => ({ className: cn(isNew && "print-digits"), style: { "--chars": text.length } as CSSProperties });
              const saldo = formatCoins(line.balance_after);
              const debit = e.side === "debit" ? `${signSymbol("debit")}${formatCoins(e.coins)}` : "";
              const credit = e.side === "credit" ? `${signSymbol("credit")}${formatCoins(e.coins)}` : "";
              return (
                <tr key={line.id} data-line-id={line.id} data-kind={line.kind} className={cn("align-top", isNew && "print-line")}>
                  <td className="whitespace-nowrap py-2.5 pr-2 font-mono text-[13px] tabular-nums text-muted">{short(line.created_at)}</td>
                  <td className="py-2.5 pr-2">
                    <span className="[overflow-wrap:anywhere]">{note(line)}</span>
                    {who ? (
                      <span className={cn("mt-0.5 block text-[13px] leading-tight text-muted underline decoration-2 underline-offset-4", underline[who.slot])}>
                        {t("by", { name: who.name })}
                      </span>
                    ) : null}
                  </td>
                  <td className="py-2.5 pr-2 text-right font-mono text-[13px] tabular-nums text-debit">
                    {debit ? <span {...typed(debit)}>{debit}</span> : e.side === "none" ? <span className="text-muted">0</span> : null}
                  </td>
                  <td className="py-2.5 pr-2 text-right font-mono text-[13px] tabular-nums text-credit">
                    {credit ? <span {...typed(credit)}>{credit}</span> : null}
                  </td>
                  <td className="whitespace-nowrap py-2.5 text-right font-mono text-[13px] tabular-nums">
                    <span {...typed(saldo)}>{saldo}</span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}

      <div ref={end} className="mt-4 min-h-6">
        {hasMore ? (
          more === "error" ? (
            <p role="alert" className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm text-debit">
              {t("loadError")}
              <Button size="sm" variant="secondary" onClick={onLoadMore}>
                {t("retry")}
              </Button>
            </p>
          ) : (
            <Button size="sm" variant="secondary" onClick={onLoadMore} disabled={more === "loading"}>
              {more === "loading" ? <CircleNotch aria-hidden className="size-4 animate-spin" /> : null}
              {more === "loading" ? t("loading") : t("loadMore")}
            </Button>
          )
        ) : lines.length > 0 ? (
          <p className="text-sm text-muted">{t("end")}</p>
        ) : null}
      </div>

      <p role="status" aria-live="polite" className="sr-only">
        {lastPrinted ? t("printed", { note: note(lastPrinted), saldo: formatCoins(balance) }) : ""}
      </p>
    </section>
  );
}
