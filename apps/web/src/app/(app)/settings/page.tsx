import type { Metadata } from "next";
import { cookies } from "next/headers";
import { getTranslations } from "next-intl/server";
import { ThemeToggle } from "@/components/theme-toggle";
import { LogoutButton } from "@/components/shell/logout-button";
import { getCurrentUser } from "@/lib/auth/session";
import { parseTheme, THEME_COOKIE } from "@/lib/theme";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Settings");
  return { title: t("title") };
}

export default async function SettingsPage() {
  const [t, user, jar] = await Promise.all([getTranslations("Settings"), getCurrentUser(), cookies()]);
  return (
    <>
      <h1 className="text-3xl font-semibold tracking-tight">{t("title")}</h1>

      <section aria-labelledby="account" className="mt-8 max-w-xl">
        <h2 id="account" className="text-lg font-semibold tracking-tight">
          {t("account")}
        </h2>
        <dl className="mt-3 divide-y divide-rule border-y border-rule">
          {[
            [t("name"), user?.display_name],
            [t("email"), user?.email],
            [t("timezone"), user?.timezone],
          ].map(([label, value]) => (
            <div key={label} className="grid grid-cols-[7rem_1fr] gap-x-4 py-3 text-[15px]">
              <dt className="text-muted">{label}</dt>
              <dd className="min-w-0 break-words">{value}</dd>
            </div>
          ))}
        </dl>
      </section>

      <section aria-labelledby="appearance" className="mt-8 max-w-xl">
        <h2 id="appearance" className="mb-3 text-lg font-semibold tracking-tight">
          {t("appearance")}
        </h2>
        <ThemeToggle initial={parseTheme(jar.get(THEME_COOKIE)?.value)} />
      </section>

      <div className="mt-10 max-w-xl">
        <LogoutButton block className="sm:w-auto" />
      </div>
    </>
  );
}
