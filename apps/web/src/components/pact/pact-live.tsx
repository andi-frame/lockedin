"use client";

import { useTranslations } from "next-intl";
import type { ReactNode } from "react";
import type { components } from "@/lib/api/schema";
import { PactCalendar, type CalendarMember } from "./pact-calendar";
import { Passbook } from "./passbook";
import { usePactLive } from "./use-pact-live";

type LedgerPage = components["schemas"]["LedgerPage"];
type CheckIn = components["schemas"]["CheckIn"];

/**
 * The running pact: the coin book on the left and, beside it (below on a phone), the calendar and
 * whatever the server page puts in `children` (members, terms). One hook feeds both, so a miss that
 * the worker prints shows up in the book and on the calendar in the same refresh.
 */
export function PactLive({
  pactId,
  rate,
  timeZone,
  startsOn,
  endsOn,
  today,
  me,
  members,
  initial,
  children,
}: {
  pactId: string;
  rate: number;
  timeZone: string;
  startsOn: string;
  endsOn: string;
  today: string;
  me: string;
  members: CalendarMember[];
  initial: { ledger: LedgerPage; checkIns: CheckIn[] };
  children: ReactNode;
}) {
  const t = useTranslations("PactPage");
  const live = usePactLive(pactId, initial);
  const people = Object.fromEntries(members.map((m) => [m.id, { name: m.name, slot: m.slot }]));

  return (
    <div className="mt-8 grid items-start gap-x-10 gap-y-10 lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
      <Passbook
        title={t("ledgerTitle")}
        rate={rate}
        timeZone={timeZone}
        lines={live.lines}
        balance={live.balance}
        fresh={live.fresh}
        lastPrinted={live.lastPrinted}
        members={people}
        hasMore={live.hasMore}
        more={live.more}
        onLoadMore={live.loadMore}
      />
      <div className="flex min-w-0 flex-col gap-10">
        <section aria-labelledby="calendar">
          <h2 id="calendar" className="mb-3 text-lg font-semibold tracking-tight">
            {t("calendarTitle")}
          </h2>
          <PactCalendar pactId={pactId} startsOn={startsOn} endsOn={endsOn} today={today} me={me} members={members} checkIns={live.checkIns} />
        </section>
        {children}
      </div>
    </div>
  );
}
