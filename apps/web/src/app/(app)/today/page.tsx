import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { getCurrentUser } from "@/lib/auth/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Today");
  return { title: t("title") };
}

// The real screen (countdown, check-in panel, passbook) is PLAN 6.2; until then a new account sees
// who it is signed in as and an honest empty state.
export default async function TodayPage() {
  const [t, user] = await Promise.all([getTranslations("Today"), getCurrentUser()]);
  return (
    <>
      <h1 className="text-3xl font-semibold tracking-tight">{t("title")}</h1>
      <p className="mt-2 text-lg">{t("greeting", { name: user?.display_name ?? "" })}</p>
      <p className="mt-6 max-w-prose text-muted">{t("empty")}</p>
    </>
  );
}
