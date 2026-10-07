// The four places the signed-in app is organised around. The rail (desktop) and the tab bar
// (mobile) render the same list so the two never disagree.
export const navItems = [
  { key: "today", href: "/today" },
  { key: "pacts", href: "/pacts" },
  { key: "review", href: "/review" },
  { key: "settings", href: "/settings" },
] as const;

export type NavKey = (typeof navItems)[number]["key"];

/** A section stays highlighted on its sub-pages (`/pacts/9f2` lights up Kontrak). */
export function isActive(pathname: string, href: string): boolean {
  const path = pathname.length > 1 ? pathname.replace(/\/+$/, "") : pathname;
  return path === href || path.startsWith(`${href}/`);
}
