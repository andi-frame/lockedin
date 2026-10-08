import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import Link from "next/link";
import { notFound } from "next/navigation";
import { ProofForm } from "@/components/proof/proof-form";
import { StatusChip } from "@/components/status-chip";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/errors";
import { serverApi } from "@/lib/api/server";
import { unwrap } from "@/lib/api/unwrap";
import { getCurrentUser } from "@/lib/auth/session";
import { isDate } from "@/lib/pact/dates";
import type { ProofDoc } from "@/lib/proof/doc";
import type { TrayItem } from "@/lib/proof/tray";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Proof");
  return { title: t("pageTitle") };
}

const missing = (err: unknown) => err instanceof ApiError && (err.status === 404 || err.status === 400);

// The doer's page for one day. While the check-in can still take a proof (`submit`, or
// `edit_proof` after sending) it holds the editor; otherwise it says where the day stands. The
// read-only proof with its history and the reviewer's actions are task 6.5, on this same route.
export default async function DayPage({ params }: { params: Promise<{ id: string; date: string }> }) {
  const { id, date } = await params;
  if (!isDate(date)) notFound();
  const [t, user, api] = await Promise.all([getTranslations("Proof"), getCurrentUser(), serverApi()]);
  if (!user) return null;

  const [pact, list] = await Promise.all([
    unwrap(api.GET("/pacts/{pactId}", { params: { path: { pactId: id } } })),
    unwrap(api.GET("/pacts/{pactId}/check-ins", { params: { path: { pactId: id }, query: { from: date, to: date } } })),
  ]).catch((err) => {
    if (missing(err)) notFound();
    throw err;
  });
  const mine = list.items.find((c) => c.member_id === user.id);
  if (!mine) notFound(); // nothing is scheduled for this person that day (a non-member got 404 above)

  const detail = await unwrap(api.GET("/check-ins/{checkInId}", { params: { path: { checkInId: mine.id } } }));
  const canEdit = detail.my_actions.includes("submit") || detail.my_actions.includes("edit_proof");
  const editing = detail.my_actions.includes("edit_proof");
  const terms = pact.terms.members[user.id];

  if (!canEdit || !terms) {
    return (
      <section className="max-w-xl">
        <h1 className="text-3xl font-semibold tracking-tight">{t("pageTitle")}</h1>
        <div className="mt-4">
          <StatusChip status={detail.check_in.status} />
        </div>
        <p className="mb-6 mt-3 text-[15px] text-muted">{t("closed")}</p>
        <Button asChild>
          <Link href="/today">{t("backToToday")}</Link>
        </Button>
      </section>
    );
  }

  const attachments: TrayItem[] = detail.attachments.map((a) => ({
    localId: a.id,
    attachmentId: a.id,
    name: a.mime ?? a.kind,
    kind: a.kind,
    bytes: a.bytes ?? 0,
    thumb: a.urls?.thumb ?? a.urls?.poster ?? null,
    state: "ready",
  }));

  return (
    <>
      <h1 className="mb-8 text-3xl font-semibold tracking-tight">{editing ? t("editTitle") : t("pageTitle")}</h1>
      <ProofForm
        pactId={pact.id}
        checkInId={detail.check_in.id}
        timezone={pact.timezone}
        commitment={terms.commitment}
        rules={{ minAttachments: terms.evidence.min_attachments, minWords: terms.evidence.min_words }}
        cutoffAt={detail.check_in.cutoff_at}
        submitDeadline={detail.check_in.submit_deadline}
        serverNow={new Date().toISOString()}
        initialDoc={(detail.proof?.body_doc as ProofDoc | undefined) ?? null}
        initialAttachments={attachments}
        editing={editing}
      />
    </>
  );
}
