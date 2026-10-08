import { Bell } from "@phosphor-icons/react/dist/ssr";
import { useTranslations } from "next-intl";
import Link from "next/link";

/** The way into the inbox, with the unread count on it. The count is the server's (`/notifications`), never counted here. */
export function NotificationBell({ unread }: { unread: number }) {
  const t = useTranslations("Nav");
  return (
    <Link
      href="/notifications"
      aria-label={unread > 0 ? t("notificationsUnread", { count: unread }) : t("notifications")}
      className="relative inline-flex size-11 items-center justify-center rounded-control text-cover-muted outline-offset-2 transition-colors duration-150 hover:bg-cover-ink/8 hover:text-cover-ink focus-visible:outline-2 focus-visible:outline-cover-ink lg:size-9"
    >
      <Bell aria-hidden weight={unread > 0 ? "fill" : "bold"} className="size-5" />
      {unread > 0 ? (
        <span
          aria-hidden
          className="absolute -right-0.5 -top-0.5 min-w-[18px] rounded-control bg-cover-ink px-1 text-center font-mono text-[11px] font-semibold leading-[18px] text-cover"
        >
          {unread > 99 ? "99+" : unread}
        </span>
      ) : null}
    </Link>
  );
}
