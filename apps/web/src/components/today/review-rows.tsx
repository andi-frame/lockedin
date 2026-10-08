import { useTranslations } from "next-intl";
import Link from "next/link";
import { Countdown } from "@/components/countdown";
import { MemberLine } from "@/components/member-line";
import type { components } from "@/lib/api/schema";
import { memberSlot } from "@/lib/member";

type Item = components["schemas"]["ReviewQueueItem"];

/** Submissions waiting for the signed-in person's decision, soonest review deadline first. A row opens the check-in itself. */
export function ReviewRows({ items, total, serverNow }: { items: Item[]; total: number; serverNow: string }) {
  const t = useTranslations("Today");
  if (total === 0) return null;
  return (
    <section aria-labelledby="to-review" className="border-t border-rule pt-5">
      <div className="flex items-baseline justify-between gap-4">
        <h2 id="to-review" className="text-lg font-semibold tracking-tight">
          {t("toReview", { n: total })}
        </h2>
        <Link href="/review" className="text-sm font-medium text-teal-text underline-offset-4 hover:underline">
          {t("seeAll")}
        </Link>
      </div>
      <ul className="mt-3 divide-y divide-rule border-y border-rule">
        {items.map((it) => (
          <li key={it.check_in.id}>
            <Link
              href={`/pacts/${it.check_in.pact_id}/days/${it.check_in.local_date}?of=${it.member.user_id}`}
              className="flex min-h-14 flex-wrap items-center justify-between gap-x-4 gap-y-1 py-3 outline-offset-4 hover:bg-sunken focus-visible:outline-2 focus-visible:outline-ring sm:px-2"
            >
              <span className="min-w-0">
                <MemberLine name={it.member.display_name} slot={memberSlot(it.member.user_id, [it.member.user_id, it.check_in.reviewer_id])} />
                <span className="mt-1 block truncate text-sm text-muted">{it.pact_title}</span>
              </span>
              {it.check_in.review_deadline ? <Countdown until={it.check_in.review_deadline} serverNow={serverNow} /> : null}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
