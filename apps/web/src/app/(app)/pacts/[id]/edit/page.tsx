import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { notFound, redirect } from "next/navigation";
import { Wizard } from "@/components/pact/wizard";
import { ApiError } from "@/lib/api/errors";
import { serverApi } from "@/lib/api/server";
import { unwrap } from "@/lib/api/unwrap";
import { getCurrentUser } from "@/lib/auth/session";
import { draftFromPact } from "@/lib/pact/draft";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Wizard");
  return { title: t("editTitle") };
}

export default async function EditPactPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const [t, user, api] = await Promise.all([getTranslations("Wizard"), getCurrentUser(), serverApi()]);
  if (!user) return null;
  const pact = await unwrap(api.GET("/pacts/{pactId}", { params: { path: { pactId: id } } })).catch((err) => {
    if (err instanceof ApiError && (err.status === 404 || err.status === 400)) notFound();
    throw err;
  });
  // Terms are frozen once both have signed (SPEC §3); until then either member may edit.
  if (pact.status !== "draft" && pact.status !== "proposed") redirect(`/pacts/${id}`);
  return (
    <>
      <h1 className="mb-8 text-3xl font-semibold tracking-tight">{t("editTitle")}</h1>
      <Wizard me={{ id: user.id, displayName: user.display_name }} initial={draftFromPact(pact)} pact={pact} />
    </>
  );
}
