import { getTranslations } from "next-intl/server";

// Placeholder until the auth pages and app shell land (PLAN 5.3).
export default async function Home() {
  const t = await getTranslations("Home");
  return (
    <main className="mx-auto flex min-h-dvh max-w-prose flex-col justify-center gap-3 px-4">
      <h1 className="text-4xl font-semibold tracking-tight">{t("name")}</h1>
      <p className="text-lg">{t("tagline")}</p>
      <p className="text-sm opacity-70">{t("building")}</p>
    </main>
  );
}
