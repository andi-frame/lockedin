import { useTranslations } from "next-intl";
import { Countdown } from "@/components/countdown";
import { NotificationBell } from "@/components/shell/notification-bell";

/**
 * The cover-teal header band: today's date (the one place the highlighter is used) and how long
 * until the next check-in closes, in fixed cells. The h1 is here so the page has one landmark.
 */
export function DeadlineBand({
  dateLabel,
  serverNow,
  next,
  hasPacts,
  unread,
}: {
  dateLabel: string;
  serverNow: string;
  next: { until: string; pactTitle: string } | null;
  hasPacts: boolean;
  unread: number;
}) {
  const t = useTranslations("Today");
  return (
    <header className="guilloche rounded-panel bg-cover px-5 py-6 text-cover-ink sm:px-7 sm:py-7">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <h1 className="text-3xl font-semibold tracking-tight">{t("title")}</h1>
        <div className="flex items-center gap-3">
          <span className="rounded-control bg-today px-2.5 py-1 text-sm font-medium text-today-ink">{dateLabel}</span>
          {/* The rail carries the bell from `lg` up; on a phone the band is its place. */}
          <span className="lg:hidden">
            <NotificationBell unread={unread} />
          </span>
        </div>
      </div>
      {next ? (
        <div className="mt-6">
          <p className="text-sm text-cover-muted">{t("nextDeadline", { title: next.pactTitle })}</p>
          <Countdown until={next.until} serverNow={serverNow} onCover size="lg" className="mt-2" />
        </div>
      ) : hasPacts ? (
        <p className="mt-6 text-lg">{t("allDone")}</p>
      ) : null}
    </header>
  );
}
