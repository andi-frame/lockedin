"use client";

import { CaretLeft, CaretRight } from "@phosphor-icons/react";
import { useFormatter, useTranslations } from "next-intl";
import Link from "next/link";
import { useState } from "react";
import { statusIcon, type CheckInStatus } from "@/components/status-chip";
import { cn } from "@/lib/cn";
import type { components } from "@/lib/api/schema";
import type { MemberSlot } from "@/lib/member";
import { dayCells, monthsBetween, shiftMonth, startMonth, weeksOf } from "@/lib/pact/calendar";
import { Button } from "@/components/ui/button";

type CheckIn = components["schemas"]["CheckIn"];

export type CalendarMember = { id: string; name: string; slot: MemberSlot };

// The symbol says what happened; its ink repeats it (never alone). The member is told apart by the
// bar under the symbol in their fixed line colour, and by which side of the cell they sit on.
// On the yellow of today the ink is the cell's own: these colours are tuned for paper (pale on yellow in the dark theme).
const ink: Record<CheckInStatus, string> = {
  open: "text-muted",
  submitted: "text-teal-text",
  approved: "text-stamp-text",
  auto_approved: "text-teal-text",
  rejected: "text-stamp-text",
  disputed: "text-ink",
  missed: "text-debit",
  rest: "text-muted",
};
const bar: Record<MemberSlot, string> = { 0: "bg-member-a", 1: "bg-member-b" };

/**
 * A month grid of the pact: for each day, both members' check-in status as a symbol plus colour.
 * Today wears the highlighter. A day links to its page, which shows both members' check-ins for it.
 */
export function PactCalendar({
  pactId,
  startsOn,
  endsOn,
  today,
  me,
  members,
  checkIns,
}: {
  pactId: string;
  startsOn: string;
  endsOn: string;
  today: string;
  me: string;
  members: CalendarMember[];
  checkIns: CheckIn[];
}) {
  const t = useTranslations("Calendar");
  const ts = useTranslations("Status");
  const f = useFormatter();
  const months = monthsBetween(startsOn, endsOn);
  const [month, setMonth] = useState(() => startMonth(today, startsOn, endsOn));
  const at = months.indexOf(month);

  const monthLabel = f.dateTime(new Date(`${month}-01T00:00:00Z`), { month: "long", year: "numeric", timeZone: "UTC" });
  const weeks = weeksOf(month);
  const ids = members.map((m) => m.id);
  const cells = dayCells(weeks.flat(), checkIns, ids, { startsOn, endsOn, today, month });
  const byId = new Map(members.map((m) => [m.id, m]));
  const long = (date: string) => f.dateTime(new Date(`${date}T00:00:00Z`), { dateStyle: "long", timeZone: "UTC" });
  const weekday = (date: string) => f.dateTime(new Date(`${date}T00:00:00Z`), { weekday: "short", timeZone: "UTC" });
  const used = [...new Set(checkIns.map((c) => c.status))];

  return (
    <div>
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-base font-semibold capitalize tracking-tight" aria-live="polite">
          {monthLabel}
        </h3>
        <div className="flex gap-1">
          <Button variant="ghost" size="icon-sm" aria-label={t("prev")} disabled={at <= 0} onClick={() => setMonth(shiftMonth(month, -1))}>
            <CaretLeft aria-hidden weight="bold" className="size-4" />
          </Button>
          <Button variant="ghost" size="icon-sm" aria-label={t("next")} disabled={at < 0 || at >= months.length - 1} onClick={() => setMonth(shiftMonth(month, 1))}>
            <CaretRight aria-hidden weight="bold" className="size-4" />
          </Button>
        </div>
      </div>

      <table className="mt-3 w-full table-fixed border-separate border-spacing-0.5 text-center" data-testid="calendar">
        <caption className="sr-only">{t("grid", { month: monthLabel })}</caption>
        <thead>
          <tr>
            {weeks[0]?.map((d) => (
              <th key={d} scope="col" className="pb-1 text-[13px] font-medium capitalize text-muted">
                {weekday(d)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {weeks.map((week, wi) => (
            <tr key={week[0]}>
              {cells.slice(wi * 7, wi * 7 + 7).map((cell) => {
                const readable = `${long(cell.date)}: ${
                  cell.marks.length === 0
                    ? t("noCheckIn")
                    : cell.marks.map((m) => t("mark", { name: byId.get(m.memberId)?.name ?? "", status: ts(m.status).toLowerCase() })).join(", ")
                }`;
                // The day opens on the viewer's own check-in, or on the other member's when the
                // viewer has none that day (a backer who does not commit).
                const mine = cell.marks.find((m) => m.memberId === me);
                const target = mine ?? cell.marks[0];
                const body = (
                  <>
                    <span className="sr-only">{readable}</span>
                    <span aria-hidden className="block text-left font-mono text-xs tabular-nums leading-none">
                      {Number(cell.date.slice(8))}
                    </span>
                    <span aria-hidden className="mt-1 flex min-h-5 items-end justify-center gap-1.5">
                      {cell.marks.map((m) => {
                        const Icon = statusIcon(m.status);
                        const slot = byId.get(m.memberId)?.slot ?? 0;
                        return (
                          <span key={m.memberId} data-status={m.status} className="flex flex-col items-center gap-px">
                            <Icon weight="bold" className={cn("size-4", cell.today ? "text-today-ink" : ink[m.status])} />
                            <span className={cn("h-0.5 w-3.5 rounded-full", bar[slot])} />
                          </span>
                        );
                      })}
                    </span>
                  </>
                );
                const frame = cn(
                  "block min-h-[3.25rem] rounded-control px-1 pb-1 pt-1.5",
                  cell.outside && "opacity-35",
                  !cell.inPact && !cell.outside && "text-muted",
                  cell.today ? "bg-today text-today-ink" : cell.inPact && !cell.outside ? "bg-surface ring-1 ring-inset ring-rule" : null,
                );
                return (
                  <td key={cell.date} data-date={cell.date} data-today={cell.today || undefined} className="p-0 align-top">
                    {target && cell.inPact ? (
                      <Link href={`/pacts/${pactId}/days/${cell.date}${mine ? "" : `?of=${target.memberId}`}`} className={cn(frame, "hover:ring-rule-strong")}>
                        {body}
                      </Link>
                    ) : (
                      <div className={frame}>{body}</div>
                    )}
                  </td>
                );
              })}
            </tr>
          ))}
        </tbody>
      </table>

      {used.length > 0 ? (
        <ul aria-label={t("legend")} className="mt-3 flex flex-wrap gap-x-4 gap-y-1.5 text-[13px] text-muted">
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
      ) : null}
    </div>
  );
}
