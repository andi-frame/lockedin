import { Plus } from "@phosphor-icons/react/dist/ssr";
import type { Metadata } from "next";
import { getFormatter, getTranslations } from "next-intl/server";
import Link from "next/link";
import { DeadlineBand } from "@/components/today/deadline-band";
import { MiniPassbook } from "@/components/today/mini-passbook";
import { PactSection } from "@/components/today/pact-section";
import { ReviewRows } from "@/components/today/review-rows";
import { Button } from "@/components/ui/button";
import { serverApi } from "@/lib/api/server";
import { unwrap } from "@/lib/api/unwrap";
import { getCurrentUser, getUnreadCount } from "@/lib/auth/session";
import { todayIn } from "@/lib/pact/dates";
import { groupToday, nextDeadline } from "@/lib/today";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Today");
  return { title: t("title") };
}

// One call (`getToday`) plus the pact details that carry each commitment and the coin rate, and the
// first few submissions waiting for review. Sections follow the API's order: nearest deadline first.
export default async function TodayPage() {
  const [t, format, user, api] = await Promise.all([getTranslations("Today"), getFormatter(), getCurrentUser(), serverApi()]);
  if (!user) return null;

  const [today, queue, unread] = await Promise.all([
    unwrap(api.GET("/today")),
    unwrap(api.GET("/review-queue", { params: { query: { limit: 5 } } })),
    getUnreadCount(),
  ]);
  const details = await Promise.all(today.pacts.map((p) => unwrap(api.GET("/pacts/{pactId}", { params: { path: { pactId: p.pact_id } } }))));
  const byId = new Map(details.map((p) => [p.id, p]));

  const now = new Date(today.server_time);
  const dateLabel = format.dateTime(now, { weekday: "long", day: "numeric", month: "long", timeZone: user.timezone });
  const sections = groupToday(today);

  return (
    <>
      <DeadlineBand dateLabel={dateLabel} serverNow={today.server_time} next={nextDeadline(today)} hasPacts={sections.length > 0} unread={unread} />

      {sections.length === 0 && today.review_queue_count === 0 ? (
        <div className="mt-8 max-w-prose">
          <p className="text-muted">{t("empty")}</p>
          <Button asChild className="mt-5">
            <Link href="/pacts/new">
              <Plus aria-hidden weight="bold" className="size-4" />
              {t("newPact")}
            </Link>
          </Button>
        </div>
      ) : (
        <div className="mt-8 grid gap-10 lg:grid-cols-2 lg:gap-12">
          <div className="flex min-w-0 flex-col gap-6">
            {sections.map((s) => {
              const pact = byId.get(s.pact.pact_id);
              return pact ? (
                <PactSection key={s.pact.pact_id} section={s} pact={pact} me={user.id} serverNow={today.server_time} today={todayIn(pact.timezone, now)} />
              ) : null;
            })}
            <ReviewRows items={queue.items} total={today.review_queue_count} serverNow={today.server_time} />
          </div>
          <div className="flex min-w-0 flex-col gap-10">
            {sections.map((s) => {
              const pact = byId.get(s.pact.pact_id);
              return pact ? <MiniPassbook key={s.pact.pact_id} pact={s.pact} rate={pact.terms.coin_rate_idr} timeZone={pact.timezone} /> : null;
            })}
          </div>
        </div>
      )}
    </>
  );
}
