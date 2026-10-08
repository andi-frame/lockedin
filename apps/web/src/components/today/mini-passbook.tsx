import { useFormatter, useTranslations } from "next-intl";
import Link from "next/link";
import { Amount } from "@/components/amount";
import type { components } from "@/lib/api/schema";
import { formatCoins } from "@/lib/format";
import { ledgerEntry } from "@/lib/today";

type TodayPact = components["schemas"]["TodayPact"];

/**
 * The last three lines of the pot and its saldo, printed like the passbook: the balance is always
 * the sum of the lines (SPEC §6). The full passbook is on the pact page (task 6.4).
 */
export function MiniPassbook({ pact, rate, timeZone }: { pact: TodayPact; rate: number; timeZone: string }) {
  const t = useTranslations("Ledger");
  const f = useFormatter();
  const short = (iso: string) => f.dateTime(new Date(iso), { day: "numeric", month: "short", timeZone });
  const day = (d: string) => f.dateTime(new Date(`${d}T00:00:00Z`), { day: "numeric", month: "short", timeZone: "UTC" });

  return (
    <section aria-labelledby={`book-${pact.pact_id}`}>
      <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
        <h2 id={`book-${pact.pact_id}`} className="min-w-0 text-lg font-semibold tracking-tight [overflow-wrap:anywhere]">
          {pact.title}
        </h2>
        <Link href={`/pacts/${pact.pact_id}`} className="text-sm font-medium text-teal-text underline-offset-4 hover:underline">
          {t("open")}
        </Link>
      </div>

      <p className="mt-3 text-sm text-muted">{t("saldo")}</p>
      <Amount coins={pact.balance} direction="balance" rate={rate} size="xl" />

      {pact.recent_ledger.length === 0 ? (
        <p className="mt-4 text-[15px] text-muted">{t("empty")}</p>
      ) : (
        <table className="mt-4 w-full border-collapse text-sm">
          <caption className="sr-only">{t("caption", { title: pact.title })}</caption>
          <thead>
            <tr className="border-b border-rule-strong text-left text-[13px] text-muted">
              <th scope="col" className="py-2 pr-2 font-medium">{t("date")}</th>
              <th scope="col" className="py-2 pr-2 font-medium">{t("note")}</th>
              <th scope="col" className="py-2 pr-2 text-right font-medium">{t("amount")}</th>
              <th scope="col" className="py-2 text-right font-medium">{t("saldo")}</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-rule">
            {pact.recent_ledger.map((line) => {
              const e = ledgerEntry(line);
              return (
                <tr key={line.id} className="align-top">
                  <td className="whitespace-nowrap py-2.5 pr-2 font-mono tabular-nums text-muted">{short(line.created_at)}</td>
                  <td className="py-2.5 pr-2">{t(e.label.replace("Ledger.", ""), { date: e.params.date ? day(e.params.date) : "" })}</td>
                  <td className="py-2.5 pr-2 text-right">
                    {e.side === "none" ? (
                      <span className="font-mono tabular-nums text-muted">0</span>
                    ) : (
                      <Amount coins={e.coins} direction={e.side} size="sm" className="items-end" />
                    )}
                  </td>
                  <td className="whitespace-nowrap py-2.5 text-right font-mono tabular-nums">{formatCoins(line.balance_after)}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      )}
    </section>
  );
}
