import { getTranslations } from "next-intl/server";
import { redirect } from "next/navigation";
import type { ReactNode } from "react";
import { LogoutButton } from "@/components/shell/logout-button";
import { NavRail, TabBar } from "@/components/shell/nav";
import { NotificationBell } from "@/components/shell/notification-bell";
import { Wordmark } from "@/components/wordmark";
import { getCurrentUser, getUnreadCount } from "@/lib/auth/session";

// The signed-in shell. src/proxy.ts has only checked that a session cookie exists; this asks the
// API who the visitor is, so an expired session ends at the login page rather than a broken screen.
export default async function AppLayout({ children }: { children: ReactNode }) {
  const user = await getCurrentUser();
  if (!user) redirect("/login");
  const [t, unread] = await Promise.all([getTranslations("Nav"), getUnreadCount()]);

  return (
    <div className="min-h-dvh lg:pl-60">
      <a
        href="#main"
        className="sr-only z-50 rounded-control bg-surface px-3 py-2 font-medium text-ink focus:not-sr-only focus:fixed focus:left-3 focus:top-3"
      >
        {t("skip")}
      </a>

      <aside className="guilloche fixed inset-y-0 left-0 hidden w-60 flex-col bg-cover px-3 py-5 text-cover-ink lg:flex">
        <div className="flex items-center justify-between px-3">
          <Wordmark />
          <NotificationBell unread={unread} />
        </div>
        <div className="mt-8">
          <NavRail />
        </div>
        <div className="mt-auto border-t border-cover-ink/15 px-3 pt-4">
          <p className="truncate text-sm font-medium">{user.display_name}</p>
          <p className="truncate text-sm text-cover-muted">{user.email}</p>
          <LogoutButton variant="ghost" size="sm" className="-ml-3 mt-2 text-cover-muted hover:bg-cover-ink/8 hover:text-cover-ink" />
        </div>
      </aside>

      <main id="main" tabIndex={-1} className="mx-auto w-full max-w-5xl px-4 pb-[calc(6rem+env(safe-area-inset-bottom))] pt-6 outline-none sm:px-8 lg:px-12 lg:pb-12 lg:pt-10">
        {children}
      </main>

      <TabBar />
    </div>
  );
}
