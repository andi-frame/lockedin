import { useFormatter, useTranslations, type DateTimeFormatOptions } from "next-intl";
import type { CSSProperties } from "react";
import { Amount } from "@/components/amount";
import { MemberLine } from "@/components/member-line";
import { statusIcon, type CheckInStatus } from "@/components/status-chip";
import { cn } from "@/lib/cn";
import { formatCoins } from "@/lib/format";
import { SAMPLE, sampleLines, type Mark } from "@/lib/landing/sample";
import { PrintOnView } from "./print-on-view";

const RATE = 1000; // rupiah per coin, as in the app's examples
const DOER_SLOT = 1;
const BACKER_SLOT = 0;

// The symbol says what happened; its ink repeats it, never alone (same as the app's calendar). On the
// yellow of today the ink is the cell's own: the status colours are tuned for paper, not for yellow.
const ink: Partial<Record<Mark, string>> = {
  approved: "text-stamp-text",
  missed: "text-debit",
  rest: "text-muted",
  submitted: "text-teal-text",
  open: "text-muted",
};
const bar = { 0: "bg-member-a", 1: "bg-member-b" } as const;
const asStatus = (m: Mark): CheckInStatus | null => (m === "approved" || m === "missed" || m === "rest" || m === "submitted" || m === "open" ? m : null);
const delay = (i: number) => ({ "--i": i }) as CSSProperties;

/**
 * One month of a made-up pact, drawn as the product draws it: a stamped calendar and the printed
 * lines under it. It is the landing page's proof, labelled as an example, and it obeys the
 * product's rule (the saldo of a line is the previous saldo plus its amount, see lib/landing).
 */
export function SamplePage({ className }: { className?: string }) {
  const t = useTranslations("Landing");
  const ts = useTranslations("Status");
  const f = useFormatter();
  const utc = (day: number, opts: DateTimeFormatOptions) => f.dateTime(new Date(Date.UTC(2026, 9, day)), { ...opts, timeZone: "UTC" });
  const lines = sampleLines();
  const saldo = lines.at(-1)?.saldo ?? 0;
  const doer = t("doerName");
  const backer = t("backerName");

  // Cells in weeks of seven, Monday first; blanks before the 1st and after the 31st.
  const cells: (number | null)[] = [...Array<null>(SAMPLE.firstWeekday).fill(null), ...SAMPLE.days.map((d) => d.day)];
  while (cells.length % 7) cells.push(null);
  const weeks = Array.from({ length: cells.length / 7 }, (_, i) => cells.slice(i * 7, i * 7 + 7));

  // Stamps arrive in date order: the index sets each one's delay. Lines follow the last stamp.
  let stamp = 0;
  const used = (["approved", "missed", "rest", "submitted", "open"] as const).filter((s) => SAMPLE.days.some((d) => d.doer === s || d.backer === s));
  const stamps = SAMPLE.days.reduce((n, d) => n + (asStatus(d.doer) ? 1 : 0) + (asStatus(d.backer) ? 1 : 0), 0);

  return (
    <PrintOnView className={cn("rounded-panel border border-rule-strong bg-surface p-4 text-ink sm:p-5", className)}>
      <h2 className="text-lg font-semibold tracking-tight">{t("sampleTitle")}</h2>
      <p className="mt-1 text-sm text-muted">{t("sampleNote")}</p>

      <div className="mt-4 flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <h3 className="text-base font-semibold capitalize tracking-tight">{utc(1, { month: "long", year: "numeric" })}</h3>
        <span className="inline-flex items-center gap-3 text-[13px]">
          <MemberLine name={backer} slot={BACKER_SLOT} role="backer" className="[&>span:first-child]:size-5 [&>span:first-child]:text-xs" />
          <MemberLine name={doer} slot={DOER_SLOT} role="doer" className="[&>span:first-child]:size-5 [&>span:first-child]:text-xs" />
        </span>
      </div>

      <table className="mt-2 w-full table-fixed border-separate border-spacing-0.5 text-center">
        <caption className="sr-only">{t("sampleCalendar")}</caption>
        <thead>
          <tr>
            {Array.from({ length: 7 }, (_, i) => (
              <th key={i} scope="col" className="pb-1 text-[13px] font-medium capitalize text-muted">
                {utc(5 + i, { weekday: "short" })}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {weeks.map((week, wi) => (
            <tr key={wi}>
              {week.map((n, ci) => {
                if (n === null) return <td key={ci} className="p-0" />;
                const d = SAMPLE.days[n - 1]!;
                const marks = [
                  { who: doer, mark: d.doer, slot: DOER_SLOT },
                  { who: backer, mark: d.backer, slot: BACKER_SLOT },
                ].filter((m) => asStatus(m.mark));
                const readable = `${utc(n, { dateStyle: "full" })}: ${marks.map((m) => `${m.who} ${ts(asStatus(m.mark)!).toLowerCase()}`).join(", ") || "-"}`;
                const inPact = n <= SAMPLE.pactEnds;
                return (
                  <td key={ci} className="p-0 align-top">
                    <div
                      data-today={d.today || undefined}
                      className={cn(
                        "min-h-12 rounded-control px-1 pb-1 pt-1 sm:min-h-[3.25rem] sm:pt-1.5",
                        d.today ? "bg-today text-today-ink" : inPact ? "bg-surface ring-1 ring-inset ring-rule" : "text-muted opacity-40",
                      )}
                    >
                      <span className="sr-only">{readable}</span>
                      <span aria-hidden className="block text-left font-mono text-xs tabular-nums leading-none">
                        {n}
                      </span>
                      <span aria-hidden className="mt-1 flex min-h-5 items-end justify-center gap-1">
                        {marks.map((m) => {
                          const Icon = statusIcon(asStatus(m.mark)!);
                          return (
                            <span key={m.who} className="sample-stamp flex flex-col items-center gap-px" style={delay(stamp++)}>
                              <Icon weight="bold" className={cn("size-4", d.today ? "text-today-ink" : ink[m.mark])} />
                              <span className={cn("h-0.5 w-3 rounded-full", bar[m.slot as 0 | 1])} />
                            </span>
                          );
                        })}
                      </span>
                    </div>
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>

      <ul className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[13px] text-muted">
        {used.map((s) => {
          const Icon = statusIcon(s);
          return (
            <li key={s} className="inline-flex items-center gap-1.5">
              <Icon aria-hidden weight="bold" className={cn("size-4", ink[s])} />
              {ts(s)}
            </li>
          );
        })}
      </ul>

      <table className="mt-5 w-full border-collapse text-[15px]">
        <caption className="sr-only">{t("sampleBook")}</caption>
        <thead>
          <tr className="border-b border-rule-strong text-left text-[13px] text-muted">
            <th scope="col" className="py-1.5 pr-2 font-medium">
              {t("colDate")}
            </th>
            <th scope="col" className="py-1.5 pr-2 font-medium">
              {t("colDesc")}
            </th>
            <th scope="col" className="px-1 py-1.5 text-right font-medium">
              <span aria-hidden className="sm:hidden">
                {t("debitShort")}
              </span>
              <span className="sr-only sm:not-sr-only">{t("colDebit")}</span>
            </th>
            <th scope="col" className="px-1 py-1.5 text-right font-medium">
              <span aria-hidden className="sm:hidden">
                {t("creditShort")}
              </span>
              <span className="sr-only sm:not-sr-only">{t("colCredit")}</span>
            </th>
            <th scope="col" className="py-1.5 pl-1 text-right font-medium">
              {t("colSaldo")}
            </th>
          </tr>
        </thead>
        <tbody>
          {lines.map((l, i) => {
            const date = utc(l.day, { day: "numeric", month: "short" });
            const who = l.kind === "doer_miss" ? { name: doer, slot: DOER_SLOT } : l.kind === "backer_miss" ? { name: backer, slot: BACKER_SLOT } : null;
            return (
              <tr key={`${l.day}-${l.kind}`} className="sample-line border-b border-rule align-top" style={delay(stamps + i * 2)}>
                <td className="py-2 pr-2 font-mono text-[13px] tabular-nums text-muted">{date}</td>
                <td className="py-2 pr-2">
                  {l.kind === "pot_initial" ? t("potInitial") : t("missedOn", { date })}
                  {who ? (
                    <span className={cn("block w-fit text-[13px] text-muted underline decoration-2 underline-offset-4", who.slot === 0 ? "decoration-member-a" : "decoration-member-b")}>
                      {t("by", { name: who.name })}
                    </span>
                  ) : null}
                </td>
                <td className="px-1 py-2 text-right font-mono tabular-nums text-debit">{l.amount < 0 ? `−${formatCoins(-l.amount)}` : null}</td>
                <td className="px-1 py-2 text-right font-mono tabular-nums text-credit">{l.amount > 0 ? `+${formatCoins(l.amount)}` : null}</td>
                <td className="py-2 pl-1 text-right font-mono tabular-nums">{formatCoins(l.saldo)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>

      <div className="mt-4 flex items-end justify-between gap-3">
        <span className="text-sm text-muted">{t("saldo")}</span>
        <Amount coins={saldo} direction="balance" rate={RATE} size="lg" className="items-end" />
      </div>
    </PrintOnView>
  );
}
