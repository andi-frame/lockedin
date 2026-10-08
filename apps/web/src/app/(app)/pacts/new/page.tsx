import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { Wizard } from "@/components/pact/wizard";
import { getCurrentUser } from "@/lib/auth/session";
import { newDraft } from "@/lib/pact/draft";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Wizard");
  return { title: t("pageTitle") };
}

export default async function NewPactPage() {
  const [t, user] = await Promise.all([getTranslations("Wizard"), getCurrentUser()]);
  // The layout has already sent a signed-out visitor away; this only narrows the type.
  if (!user) return null;
  // The first dates come from the person's own zone, computed here so the server and the browser
  // render the same form.
  const initial = newDraft({ now: new Date(), timezone: user.timezone });
  return (
    <>
      <h1 className="mb-8 text-3xl font-semibold tracking-tight">{t("pageTitle")}</h1>
      <Wizard me={{ id: user.id, displayName: user.display_name }} initial={initial} />
    </>
  );
}
