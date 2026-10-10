import type { Metadata } from "next";
import { cookies } from "next/headers";
import Link from "next/link";
import { getTranslations } from "next-intl/server";
import { EmailForm } from "@/components/settings/email-form";
import { AccountForm } from "@/components/settings/account-form";
import { PasswordForm } from "@/components/settings/password-form";
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

      {user ? (
        <section aria-labelledby="account" className="mt-8 max-w-xl">
          <h2 id="account" className="text-lg font-semibold tracking-tight">
            {t("account")}
          </h2>
          <AccountForm user={user} />
        </section>
      ) : null}

      {user ? (
        <section aria-labelledby="password" className="mt-10 max-w-xl">
          <h2 id="password" className="text-lg font-semibold tracking-tight">
            {t("passwordTitle")}
          </h2>
          <p className="mt-2 max-w-prose text-[15px]">{t("passwordBody")}</p>
          <PasswordForm />
        </section>
      ) : null}

      <section aria-labelledby="notifications" className="mt-10 max-w-xl">
        <h2 id="notifications" className="text-lg font-semibold tracking-tight">
          {t("notifications")}
        </h2>
        <p className="mt-2 max-w-prose text-[15px]">{t("notificationsBody")}</p>
        <p className="mt-3">
          <Link href="/notifications" className="text-[15px] font-medium text-teal-text underline decoration-teal-text/40 underline-offset-4 hover:decoration-teal-text">
            {t("notificationsOpen")}
          </Link>
        </p>
      </section>

      {user ? (
        <section aria-labelledby="email" className="mt-10 max-w-xl">
          <h2 id="email" className="text-lg font-semibold tracking-tight">
            {t("emailTitle")}
          </h2>
          <p className="mt-2 max-w-prose text-[15px]">{t("emailBody")}</p>
          <EmailForm off={user.email_kinds_off} />
        </section>
      ) : null}

      <section aria-labelledby="appearance" className="mt-10 max-w-xl">
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
