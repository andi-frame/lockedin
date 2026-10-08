import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import { notFound } from "next/navigation";
import { CheckInDetailView } from "@/components/checkin/detail-view";
import { ProofForm } from "@/components/proof/proof-form";
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

// One day of a pact. A day has a check-in per member, so `?of=<user id>` picks whose (it defaults to
// the viewer's own). While the viewer's own check-in can still take a proof (`submit`, or
// `edit_proof` after sending) the page holds the editor; in every other case it is the read-only
// detail: the proof, its history, every decision, and the actions the server offers.
export default async function DayPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string; date: string }>;
  searchParams: Promise<{ of?: string; edit?: string }>;
}) {
  const [{ id, date }, { of, edit }] = await Promise.all([params, searchParams]);
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
  const targetId = of && pact.members.some((m) => m.user_id === of) ? of : user.id;
  const target = list.items.find((c) => c.member_id === targetId);
  if (!target) notFound(); // nothing is scheduled for that person that day (a non-member got 404 above)
  const otherCheckIn = list.items.find((c) => c.member_id !== targetId);
  const otherMember = pact.members.find((m) => m.user_id === otherCheckIn?.member_id);

  const detail = await unwrap(api.GET("/check-ins/{checkInId}", { params: { path: { checkInId: target.id } } }));
  const canEdit = targetId === user.id && (detail.my_actions.includes("submit") || detail.my_actions.includes("edit_proof"));
  // A rejected day can still be sent again before the deadline, and it can be disputed. When both
  // are possible the detail (with the dispute button) comes first, and the editor is one link away.
  const decides = detail.my_actions.some((a) => a === "dispute" || a === "approve" || a === "reject" || a === "override" || a === "resolve_dispute");
  const showEditor = canEdit && (!decides || edit === "1");
  const editing = detail.my_actions.includes("edit_proof");
  const terms = pact.terms.members[user.id];

  if (!showEditor || !terms) {
    return (
      <CheckInDetailView
        detail={detail}
        pactId={pact.id}
        timeZone={pact.timezone}
        me={user.id}
        other={
          otherMember
            ? {
                name: otherMember.display_name,
                href: `/pacts/${pact.id}/days/${date}${targetId === user.id ? `?of=${otherMember.user_id}` : ""}`,
                isMe: otherMember.user_id === user.id,
              }
            : null
        }
        serverNow={new Date().toISOString()}
        resendHref={canEdit ? `/pacts/${pact.id}/days/${date}?edit=1` : undefined}
      />
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
