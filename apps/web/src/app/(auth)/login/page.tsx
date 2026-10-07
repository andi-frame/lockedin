import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";
import { getTranslations } from "next-intl/server";
import { LoginForm } from "@/components/auth/login-form";
import { safeNext } from "@/lib/auth/guard";
import { getCurrentUser } from "@/lib/auth/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Auth");
  return { title: t("loginTitle") };
}

export default async function LoginPage({ searchParams }: { searchParams: Promise<{ next?: string | string[] }> }) {
  const next = safeNext((await searchParams).next);
  // The proxy only saw a cookie; this is where a live session is told apart from a stale one.
  if (await getCurrentUser()) redirect(next);

  const t = await getTranslations("Auth");
  const other = next === "/today" ? "/register" : `/register?next=${encodeURIComponent(next)}`;
  return (
    <>
      <h1 className="text-3xl font-semibold tracking-tight">{t("loginTitle")}</h1>
      <p className="mb-8 mt-2 text-muted">{t("loginLead")}</p>
      <LoginForm next={next} />
      <p className="mt-8 text-sm text-muted">
        {t("noAccount")}{" "}
        <Link href={other} className="font-medium text-ink underline underline-offset-4 hover:text-primary">
          {t("register")}
        </Link>
      </p>
    </>
  );
}
