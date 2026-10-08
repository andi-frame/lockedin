import { useFormatter, useTranslations } from "next-intl";
import Link from "next/link";
import { Countdown } from "@/components/countdown";
import { MemberLine } from "@/components/member-line";
import { StatusChip } from "@/components/status-chip";
import { Button } from "@/components/ui/button";
import type { components } from "@/lib/api/schema";
import { memberSlot } from "@/lib/member";
import { clockIn } from "@/lib/pact/dates";
import { dayLabel, restLeft, type Section } from "@/lib/today";
import { RestButton } from "./rest-button";

type Pact = components["schemas"]["Pact"];
type TodayCheckIn = components["schemas"]["TodayCheckIn"];

/**
 * One active pact's "Hari ini": its commitment for the day, the status of each check-in due, and
 * what the doer can do about it. Several pacts stack as sections, nearest deadline first.
 */
export function PactSection({
  section,
  pact,
  me,
  serverNow,
  today,
}: {
  section: Section;
  pact: Pact;
  me: string;
  serverNow: string;
  /** Today's date in this pact's zone. */
  today: string;
}) {
  const t = useTranslations("Today");
  const ids = pact.members.map((m) => m.user_id);
  const commitment = pact.terms.members[me]?.commitment ?? "";
  const left = restLeft(pact, me);
  const { pact: tp } = section;

  return (
    <section aria-labelledby={`pact-${tp.pact_id}`} className="border-t border-rule pt-5 first:border-t-0 first:pt-0">
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <h2 id={`pact-${tp.pact_id}`} className="min-w-0 text-lg font-semibold tracking-tight [overflow-wrap:anywhere]">
          <Link href={`/pacts/${tp.pact_id}`} className="rounded-control outline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-ring">
            {tp.title}
          </Link>
        </h2>
        <MemberLine name={tp.partner.display_name} slot={memberSlot(tp.partner.user_id, ids)} role={tp.my_role === "doer" ? "backer" : "doer"} />
      </div>

      {section.checkIns.length === 0 ? (
        <p className="mt-3 text-[15px] text-muted">{t("nothingDue")}</p>
      ) : (
        <ul className="mt-3 divide-y divide-rule">
          {section.checkIns.map((c) => (
            <li key={c.check_in.id} className="py-4 first:pt-1 last:pb-0">
              <CheckInRow c={c} pact={pact} commitment={commitment} left={left} serverNow={serverNow} today={today} />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function CheckInRow({
  c,
  pact,
  commitment,
  left,
  serverNow,
  today,
}: {
  c: TodayCheckIn;
  pact: Pact;
  commitment: string;
  left: number;
  serverNow: string;
  today: string;
}) {
  const t = useTranslations("Today");
  const f = useFormatter();
  const tz = pact.timezone;
  const ci = c.check_in;
  const day = dayLabel(ci.local_date, today);
  const clock = (iso: string) => clockIn(iso, tz);
  const dateTime = (iso: string) => `${f.dateTime(new Date(iso), { day: "numeric", month: "short", timeZone: tz })}, ${clock(iso)}`;
  const partner = pact.members.find((m) => m.user_id === ci.reviewer_id)?.display_name ?? "";
  const beforeCutoff = Date.parse(ci.cutoff_at) > Date.parse(serverNow);

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
        <StatusChip status={ci.status} />
        {day !== "today" ? (
          <span className="font-mono text-sm tabular-nums text-muted">
            {day === "yesterday" ? t("yesterday") : f.dateTime(new Date(`${ci.local_date}T00:00:00Z`), { day: "numeric", month: "short", timeZone: "UTC" })}
          </span>
        ) : null}
      </div>
      <p className="max-w-prose text-lg font-medium leading-snug [overflow-wrap:anywhere]">{commitment}</p>

      {ci.status === "open" ? (
        <>
          <div className="flex flex-col gap-1">
            <Countdown until={ci.submit_deadline} serverNow={serverNow} size="md" />
            <p className="font-mono text-sm tabular-nums text-muted">
              {t("cutoffNote", { cutoff: clock(ci.cutoff_at), until: clock(ci.submit_deadline) })}
            </p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <Button asChild>
              <Link href={`/pacts/${pact.id}/days/${ci.local_date}`}>{t("submit")}</Link>
            </Button>
            {left > 0 && beforeCutoff ? <RestButton checkInId={ci.id} left={left} /> : null}
          </div>
        </>
      ) : null}

      {ci.status === "submitted" ? (
        <>
          <p className="max-w-prose text-[15px] text-muted">
            {t("waitingReview", { partner, until: ci.review_deadline ? dateTime(ci.review_deadline) : "" })}
            {c.word_count ? <span className="font-mono tabular-nums"> {t("words", { n: c.word_count })}</span> : null}
          </p>
          {/* A proof can be changed until the submit deadline (SPEC §5, late edits). */}
          {Date.parse(ci.submit_deadline) > Date.parse(serverNow) ? (
            <div>
              <Button variant="secondary" size="sm" asChild>
                <Link href={`/pacts/${pact.id}/days/${ci.local_date}`}>{t("editProof")}</Link>
              </Button>
            </div>
          ) : null}
        </>
      ) : null}
      {ci.status === "approved" ? <p className="text-[15px] text-muted">{t("approvedBy", { partner })}</p> : null}
      {ci.status === "auto_approved" ? <p className="text-[15px] text-muted">{t("autoApproved")}</p> : null}
      {ci.status === "missed" ? <p className="text-[15px] text-muted">{t("missed")}</p> : null}
      {ci.status === "rejected" ? <p className="text-[15px] text-muted">{t("rejected", { partner })}</p> : null}
      {ci.status === "disputed" ? <p className="text-[15px] text-muted">{t("disputed")}</p> : null}
      {ci.status === "rest" ? <p className="text-[15px] text-muted">{t("rested")}</p> : null}
    </div>
  );
}
