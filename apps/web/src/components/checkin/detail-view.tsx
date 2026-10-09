import { useFormatter, useTranslations } from "next-intl";
import Link from "next/link";
import { Countdown } from "@/components/countdown";
import { StatusChip } from "@/components/status-chip";
import type { components } from "@/lib/api/schema";
import { nextDeadline, overrideDecision } from "@/lib/checkin/timeline";
import type { ProofDoc } from "@/lib/proof/doc";
import { AttachmentGallery } from "./attachment-gallery";
import { CheckInActions } from "./check-in-actions";
import { DecisionTimeline } from "./decision-timeline";
import { ProofView } from "./proof-view";

type Detail = components["schemas"]["CheckInDetail"];

/**
 * One check-in, read-only: where it stands, what the person sent, and every decision taken on it.
 * The buttons are exactly `my_actions` from the server. An override is shown to both members at
 * the top, with its reason, because the doer cannot dispute it (SPEC §2).
 */
export function CheckInDetailView({
  detail,
  pactId,
  timeZone,
  me,
  other,
  serverNow,
  resendHref,
}: {
  detail: Detail;
  pactId: string;
  timeZone: string;
  me: string;
  /** The other member's check-in on the same day, if there is one. */
  other: { name: string; href: string; isMe: boolean } | null;
  serverNow: string;
  /** Set when the doer can still send the proof again (a rejection before the deadline). */
  resendHref?: string;
}) {
  const t = useTranslations("Detail");
  const f = useFormatter();
  const ci = detail.check_in;
  const mine = detail.member.user_id === me;
  const names = { [detail.member.user_id]: detail.member.display_name, [detail.reviewer.user_id]: detail.reviewer.display_name };
  const overridden = ci.status === "rejected" ? overrideDecision(detail.decisions) : undefined;
  const due = nextDeadline(ci);
  const date = f.dateTime(new Date(`${ci.local_date}T00:00:00Z`), { dateStyle: "full", timeZone: "UTC" });
  const proofs = detail.proof_versions;
  const latest = proofs.at(-1);

  return (
    <div className="max-w-3xl">
      <header>
        <h1 className="text-3xl font-semibold tracking-tight [overflow-wrap:anywhere]">
          {mine ? t("titleMine") : t("title", { name: detail.member.display_name })}
        </h1>
        <p className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-2 text-[15px]">
          <StatusChip status={ci.status} />
          <Link href={`/pacts/${pactId}`} className="font-medium text-teal-text underline decoration-teal-text/40 underline-offset-4 hover:decoration-teal-text">
            {detail.pact_title}
          </Link>
          <span className="font-mono text-sm tabular-nums text-muted">{date}</span>
        </p>
      </header>

      {overridden ? (
        <section role="note" aria-labelledby="overridden" className="mt-6 rounded-panel border border-stamp-text bg-stamp-tint px-4 py-3 text-stamp-text">
          <h2 id="overridden" className="text-lg font-semibold tracking-tight">
            {t("overridden")}
          </h2>
          <p className="mt-1 text-[15px]">{t("overriddenBody")}</p>
          {overridden.reason ? (
            <p className="mt-2 text-[15px] [overflow-wrap:anywhere]">
              <span className="text-[13px] font-medium">{t("reason")}: </span>
              {overridden.reason}
            </p>
          ) : null}
        </section>
      ) : null}

      {due ? (
        <p className="mt-6 flex flex-wrap items-baseline gap-x-3 gap-y-1 text-[15px]">
          <span className="text-muted">{t(`deadline_${due.kind}`)}</span>
          <Countdown until={due.at} serverNow={serverNow} />
          <span className="font-mono text-sm tabular-nums text-muted">{f.dateTime(new Date(due.at), { dateStyle: "medium", timeStyle: "short", timeZone })}</span>
        </p>
      ) : null}

      {detail.my_actions.length > 0 ? (
        <section aria-labelledby="decision" className="mt-6">
          <h2 id="decision" className="sr-only">
            {t("decisionTitle")}
          </h2>
          <CheckInActions checkInId={ci.id} actions={detail.my_actions} overridesRemaining={detail.overrides_remaining ?? null} />
        </section>
      ) : null}

      {resendHref ? (
        <p className="mt-4">
          <Link href={resendHref} className="text-[15px] font-medium text-teal-text underline decoration-teal-text/40 underline-offset-4 hover:decoration-teal-text">
            {t("resend")}
          </Link>
        </p>
      ) : null}

      <section aria-labelledby="proof" className="mt-8">
        <h2 id="proof" className="mb-3 text-lg font-semibold tracking-tight">
          {t("proofTitle")}
        </h2>
        {detail.proof ? (
          <>
            <ProofView doc={detail.proof.body_doc as ProofDoc} />
            <p className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-sm text-muted">
              <span className="font-mono tabular-nums">{t("words", { n: detail.proof.word_count })}</span>
              {proofs.length > 1 ? <span className="font-medium text-ink">{t("edited", { n: detail.proof.version })}</span> : null}
            </p>
            {proofs.length > 1 ? (
              <details className="mt-2 text-sm">
                <summary className="min-h-9 cursor-pointer py-1.5 font-medium text-teal-text">{t("versionsTitle")}</summary>
                <ol className="mt-1 divide-y divide-rule border-y border-rule">
                  {proofs.map((p) => (
                    <li key={p.id} className="flex flex-wrap justify-between gap-x-4 py-2">
                      <span>{t("version", { n: p.version, words: p.word_count })}</span>
                      <span className="font-mono text-[13px] tabular-nums text-muted">{f.dateTime(new Date(p.created_at), { dateStyle: "medium", timeStyle: "short", timeZone })}</span>
                    </li>
                  ))}
                </ol>
              </details>
            ) : null}
          </>
        ) : (
          <p className="text-[15px] text-muted">{t("noProof")}</p>
        )}
        {latest && detail.attachments.length > 0 ? (
          <div className="mt-6">
            <h3 className="mb-3 text-base font-semibold tracking-tight">{t("attachments", { n: detail.attachments.length })}</h3>
            <AttachmentGallery attachments={detail.attachments} />
          </div>
        ) : null}
      </section>

      <section aria-labelledby="timeline" className="mt-8">
        <h2 id="timeline" className="mb-3 text-lg font-semibold tracking-tight">
          {t("timelineTitle")}
        </h2>
        <DecisionTimeline decisions={detail.decisions} me={me} names={names} timeZone={timeZone} />
      </section>

      {other ? (
        <p className="mt-8">
          <Link href={other.href} className="text-[15px] font-medium text-teal-text underline decoration-teal-text/40 underline-offset-4 hover:decoration-teal-text">
            {other.isMe ? t("switchToMine") : t("switchTo", { name: other.name })}
          </Link>
        </p>
      ) : null}
    </div>
  );
}
