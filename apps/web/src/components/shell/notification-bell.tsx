import { Bell } from "@phosphor-icons/react/dist/ssr";
import { useTranslations } from "next-intl";

/**
 * The unread count, printed where the notification list will open. The list itself has no page
 * yet (PLAN 5.3 allows an inert bell), so this is a status readout and deliberately not a button.
 */
export function NotificationBell({ unread }: { unread: number }) {
  const t = useTranslations("Nav");
  return (
    <span
      role="img"
      aria-label={unread > 0 ? t("notificationsUnread", { count: unread }) : t("notifications")}
      className="relative inline-flex size-9 items-center justify-center text-cover-muted"
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
    </span>
  );
}
