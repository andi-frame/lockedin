import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Pacts");
  return { title: t("title") };
}

// The list arrives with the pact tasks in Phase 6; a new account has nothing to list yet.
export default async function PactsPage() {
  const t = await getTranslations("Pacts");
  return (
    <>
      <h1 className="text-3xl font-semibold tracking-tight">{t("title")}</h1>
      <p className="mt-6 max-w-prose text-muted">{t("empty")}</p>
    </>
  );
}
