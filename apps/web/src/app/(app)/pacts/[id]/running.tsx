import { CaretDown } from "@phosphor-icons/react/dist/ssr";
import { getFormatter, getTranslations } from "next-intl/server";
import { MemberLine } from "@/components/member-line";
import { PactLive } from "@/components/pact/pact-live";
import { PactStatusBadge } from "@/components/pact/status-badge";
import { TermsSummary } from "@/components/pact/terms-summary";
import { serverApi } from "@/lib/api/server";
import type { components } from "@/lib/api/schema";
import { unwrap } from "@/lib/api/unwrap";
import { memberSlot } from "@/lib/member";
import { todayIn } from "@/lib/pact/dates";
import { PAGE_SIZE } from "@/lib/passbook";

type Pact = components["schemas"]["Pact"];

/**
 * A pact that is live, settling or finished: the cover-teal header band, the coin book with its
 * saldo, the calendar, and the terms to look up. The agreement view (signatures, invite, sign)
 * is for the states before this one.
 */
export async function RunningPact({ pact, me }: { pact: Pact; me: string }) {
  const [t, format, api] = await Promise.all([getTranslations("PactPage"), getFormatter(), serverApi()]);
  const [ledger, checkIns] = await Promise.all([
    unwrap(api.GET("/pacts/{pactId}/ledger", { params: { path: { pactId: pact.id }, query: { limit: PAGE_SIZE } } })),
    unwrap(api.GET("/pacts/{pactId}/check-ins", { params: { path: { pactId: pact.id } } })),
  ]);

  const ids = pact.members.map((m) => m.user_id);
  const members = pact.members.map((m) => ({ id: m.user_id, name: m.display_name, slot: memberSlot(m.user_id, ids) }));
  const backer = pact.members.find((m) => m.role === "backer");
  const doer = pact.members.find((m) => m.role === "doer");
  const date = (iso: string) => format.dateTime(new Date(`${iso}T00:00:00Z`), { dateStyle: "long", timeZone: "UTC" });

  return (
    <>
      <header className="guilloche rounded-panel bg-cover px-5 py-6 text-cover-ink sm:px-7">
        <div className="flex flex-wrap items-start justify-between gap-x-4 gap-y-2">
          <h1 className="min-w-0 text-3xl font-semibold tracking-tight [overflow-wrap:anywhere]">{pact.title}</h1>
          <PactStatusBadge status={pact.status} />
        </div>
        <p className="mt-2 font-mono text-sm tabular-nums text-cover-muted">
          {date(pact.starts_on)} – {date(pact.ends_on)}
        </p>
      </header>

      <PactLive
        pactId={pact.id}
        rate={pact.terms.coin_rate_idr}
        timeZone={pact.timezone}
        startsOn={pact.starts_on}
        endsOn={pact.ends_on}
        today={todayIn(pact.timezone, new Date())}
        me={me}
        members={members}
        initial={{ ledger, checkIns: checkIns.items }}
      >
        <section aria-labelledby="members">
          <h2 id="members" className="mb-3 text-lg font-semibold tracking-tight">
            {t("members")}
          </h2>
          <ul className="divide-y divide-rule border-y border-rule">
            {pact.members.map((m) => (
              <li key={m.user_id} className="py-3">
                <MemberLine name={m.display_name} slot={memberSlot(m.user_id, ids)} role={m.role} />
              </li>
            ))}
          </ul>
        </section>

        <details className="group">
          <summary className="flex cursor-pointer list-none items-center justify-between gap-3 rounded-control py-1 text-lg font-semibold tracking-tight [&::-webkit-details-marker]:hidden">
            {t("terms")}
            <CaretDown aria-hidden weight="bold" className="size-4 text-muted transition-transform group-open:rotate-180" />
          </summary>
          <p className="mb-3 mt-1 text-sm text-muted">{t("termsHint")}</p>
          <TermsSummary terms={pact.terms} names={{ backer: backer?.display_name ?? "", doer: doer?.display_name ?? "" }} />
        </details>
      </PactLive>
    </>
  );
}
