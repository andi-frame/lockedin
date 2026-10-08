import type { Metadata } from "next";
import { getFormatter, getTranslations } from "next-intl/server";
import Link from "next/link";
import { NotificationList, type InboxRow } from "@/components/notification/notification-list";
import { serverApi } from "@/lib/api/server";
import { unwrap } from "@/lib/api/unwrap";
import { getCurrentUser } from "@/lib/auth/session";
import { checkInIdsToResolve, notificationHref, payloadRefs, type CheckInRef } from "@/lib/notification/inbox";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Notifications");
  return { title: t("title") };
}

// The inbox, newest first, a page at a time (`?cursor=`). Each row opens what it is about: the
// check-in's day when the notification names one, otherwise the pact.
export default async function NotificationsPage({ searchParams }: { searchParams: Promise<{ cursor?: string }> }) {
  const { cursor } = await searchParams;
  const [t, tKinds, format, api, user] = await Promise.all([
    getTranslations("Notifications"),
    getTranslations("Notifications.kinds"),
    getFormatter(),
    serverApi(),
    getCurrentUser(),
  ]);
  const page = await unwrap(api.GET("/notifications", { params: { query: { limit: 20, ...(cursor ? { cursor } : {}) } } }));

  // The payload carries ids only. Titles come from my pacts; the day and owner of a check-in from
  // the check-in itself. A lookup that fails costs the row its precise link, never the page.
  const [pacts, details] = await Promise.all([
    unwrap(api.GET("/pacts", { params: { query: { limit: 100 } } })).catch(() => null),
    Promise.all(
      checkInIdsToResolve(page.items).map((checkInId) =>
        unwrap(api.GET("/check-ins/{checkInId}", { params: { path: { checkInId } } })).then(
          (d) => [checkInId, d] as const,
          () => null,
        ),
      ),
    ),
  ]);
  const titles = new Map((pacts?.items ?? []).map((p) => [p.id, p.title]));
  const checkIns = new Map<string, CheckInRef & { pactTitle: string }>();
  for (const d of details) if (d) checkIns.set(d[0], { local_date: d[1].check_in.local_date, member_id: d[1].check_in.member_id, pactTitle: d[1].pact_title });

  const rows: InboxRow[] = page.items.map((n) => {
    const { pactId, checkInId } = payloadRefs(n);
    const checkIn = checkInId ? checkIns.get(checkInId) : undefined;
    return {
      id: n.id,
      kind: n.kind,
      text: tKinds(n.kind),
      pact: (pactId ? titles.get(pactId) : undefined) ?? checkIn?.pactTitle ?? t("pactFallback"),
      when: format.dateTime(new Date(n.created_at), { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", timeZone: user?.timezone }),
      href: notificationHref(n, checkIn),
      unread: !n.read_at,
    };
  });

  return (
    <>
      <h1 className="text-3xl font-semibold tracking-tight">{t("title")}</h1>
      {rows.length === 0 ? (
        <p className="mt-6 max-w-prose text-muted">{t("empty")}</p>
      ) : (
        <>
          <NotificationList rows={rows} />
          <p className="mt-4 flex gap-6 text-[15px] font-medium">
            {cursor ? (
              <Link href="/notifications" className="text-teal-text underline-offset-4 hover:underline">
                {t("newest")}
              </Link>
            ) : null}
            {page.next_cursor ? (
              <Link href={`/notifications?cursor=${encodeURIComponent(page.next_cursor)}`} className="text-teal-text underline-offset-4 hover:underline">
                {t("older")}
              </Link>
            ) : null}
          </p>
        </>
      )}
    </>
  );
}
