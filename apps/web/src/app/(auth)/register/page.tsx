import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { getLocale, getTranslations } from "next-intl/server";
import { RegisterForm } from "@/components/auth/register-form";
import { safeNext } from "@/lib/auth/guard";
import { getCurrentUser } from "@/lib/auth/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Auth");
  return { title: t("registerTitle") };
}

export default async function RegisterPage({ searchParams }: { searchParams: Promise<{ next?: string | string[] }> }) {
  const next = safeNext((await searchParams).next);
  if (await getCurrentUser()) redirect(next);

  const t = await getTranslations("Auth");
  const locale = (await getLocale()) === "en" ? "en" : "id";
  const other = next === "/today" ? "/login" : `/login?next=${encodeURIComponent(next)}`;
  return (
    <>
      <h1 className="text-3xl font-semibold tracking-tight">{t("registerTitle")}</h1>
      <p className="mb-8 mt-2 text-muted">{t("registerLead")}</p>
      <RegisterForm next={next} locale={locale} />
      <p className="mt-8 text-sm text-muted">
        {t("haveAccount")}{" "}
        <Link href={other} className="font-medium text-ink underline underline-offset-4 hover:text-primary">
          {t("login")}
        </Link>
      </p>
    </>
  );
}
