"use client";

import { CalendarCheck, GearSix, Notebook, SealCheck } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { usePathname } from "next/navigation";
import type { ComponentType } from "react";
import { cn } from "@/lib/cn";
import { isActive, navItems, type NavKey } from "@/lib/nav";

type Icon = ComponentType<{ "aria-hidden"?: boolean; weight?: "bold" | "fill"; className?: string }>;

// Kontrak is the passbook itself, so it gets the notebook; Tinjau is the stamp.
const icons: Record<NavKey, Icon> = {
  today: CalendarCheck,
  pacts: Notebook,
  review: SealCheck,
  settings: GearSix,
};

/** Desktop: the vertical list inside the cover-teal rail. */
export function NavRail() {
  const t = useTranslations("Nav");
  const pathname = usePathname();
  return (
    <nav aria-label={t("main")}>
      <ul className="flex flex-col gap-1">
        {navItems.map(({ key, href }) => {
          const active = isActive(pathname, href);
          const Icon = icons[key];
          return (
            <li key={key}>
              <Link
                href={href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "flex h-11 items-center gap-3 rounded-control px-3 text-[15px] font-medium transition-colors duration-150",
                  "outline-offset-2 focus-visible:outline-2 focus-visible:outline-cover-ink",
                  active ? "bg-cover-ink/14 text-cover-ink" : "text-cover-muted hover:bg-cover-ink/8 hover:text-cover-ink",
                )}
              >
                <Icon aria-hidden weight={active ? "fill" : "bold"} className="size-5" />
                {t(key)}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}

/** Mobile: the same four destinations as a bottom bar, thumb height, labels always visible. */
export function TabBar() {
  const t = useTranslations("Nav");
  const pathname = usePathname();
  return (
    <nav
      aria-label={t("main")}
      className="fixed inset-x-0 bottom-0 z-30 border-t border-cover-ink/15 bg-cover pb-[env(safe-area-inset-bottom)] lg:hidden"
    >
      <ul className="grid grid-cols-4">
        {navItems.map(({ key, href }) => {
          const active = isActive(pathname, href);
          const Icon = icons[key];
          return (
            <li key={key}>
              <Link
                href={href}
                aria-current={active ? "page" : undefined}
                className={cn(
                  "flex h-16 flex-col items-center justify-center gap-1 text-xs font-medium transition-colors duration-150",
                  "outline-offset-[-3px] focus-visible:outline-2 focus-visible:outline-cover-ink",
                  active ? "text-cover-ink" : "text-cover-muted hover:text-cover-ink",
                )}
              >
                <span
                  className={cn(
                    "flex h-7 w-14 items-center justify-center rounded-control transition-colors duration-150",
                    active && "bg-cover-ink/14",
                  )}
                >
                  <Icon aria-hidden weight={active ? "fill" : "bold"} className="size-[22px]" />
                </span>
                {t(key)}
              </Link>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
