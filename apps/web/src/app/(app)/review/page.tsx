import type { Metadata } from "next";
import { getFormatter, getTranslations } from "next-intl/server";
import Link from "next/link";
import { Countdown } from "@/components/countdown";
import { MemberLine } from "@/components/member-line";
import { serverApi } from "@/lib/api/server";
import { unwrap } from "@/lib/api/unwrap";
import { memberSlot } from "@/lib/member";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Review");
  return { title: t("title") };
}

// The proofs waiting for this person's decision, soonest review deadline first (the API's order).
// Each row opens the check-in itself, where the decision is taken.
export default async function ReviewPage({ searchParams }: { searchParams: Promise<{ cursor?: string }> }) {
  const { cursor } = await searchParams;
  const [t, format, api] = await Promise.all([getTranslations("Review"), getFormatter(), serverApi()]);
  const page = await unwrap(api.GET("/review-queue", { params: { query: { limit: 20, ...(cursor ? { cursor } : {}) } } }));
  const serverNow = new Date().toISOString();

  return (
    <>
      <h1 className="text-3xl font-semibold tracking-tight">{t("title")}</h1>
      {page.items.length === 0 ? (
        <p className="mt-6 max-w-prose text-muted">{t("empty")}</p>
      ) : (
        <>
          <p className="mt-2 max-w-prose text-[15px] text-muted">{t("intro")}</p>
          <ul aria-label={t("list")} className="mt-6 max-w-3xl divide-y divide-rule border-y border-rule" data-testid="review-queue">
            {page.items.map((it) => (
              <li key={it.check_in.id}>
                <Link
                  href={`/pacts/${it.check_in.pact_id}/days/${it.check_in.local_date}?of=${it.member.user_id}`}
                  className="flex min-h-16 flex-wrap items-center justify-between gap-x-6 gap-y-2 py-3 outline-offset-4 hover:bg-sunken focus-visible:outline-2 focus-visible:outline-ring sm:px-2"
                >
                  <span className="min-w-0">
                    <MemberLine name={it.member.display_name} slot={memberSlot(it.member.user_id, [it.member.user_id, it.check_in.reviewer_id])} />
                    <span className="mt-1 block text-sm text-muted [overflow-wrap:anywhere]">
                      {it.pact_title}
                      <span className="font-mono tabular-nums">
                        {" · "}
                        {format.dateTime(new Date(`${it.check_in.local_date}T00:00:00Z`), { day: "numeric", month: "short", timeZone: "UTC" })}
                        {" · "}
                        {t("words", { n: it.word_count })}
                        {" · "}
                        {t("files", { n: it.attachment_count })}
                      </span>
                    </span>
                  </span>
                  {it.check_in.review_deadline ? (
                    <span className="flex flex-col items-start gap-0.5 sm:items-end">
                      <span className="text-[13px] text-muted">{t("dueLabel")}</span>
                      <Countdown until={it.check_in.review_deadline} serverNow={serverNow} />
                    </span>
                  ) : null}
                </Link>
              </li>
            ))}
          </ul>
          {page.next_cursor ? (
            <p className="mt-4">
              <Link href={`/review?cursor=${encodeURIComponent(page.next_cursor)}`} className="text-[15px] font-medium text-teal-text underline-offset-4 hover:underline">
                {t("next")}
              </Link>
            </p>
          ) : null}
        </>
      )}
    </>
  );
}
