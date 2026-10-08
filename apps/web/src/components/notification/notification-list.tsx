"use client";

import {
  Alarm,
  ArrowUUpLeft,
  CalendarCheck,
  CalendarX,
  CheckCircle,
  FileText,
  HandCoins,
  HourglassMedium,
  Moon,
  PencilSimpleLine,
  Prohibit,
  Receipt,
  Scales,
  SealCheck,
  Timer,
  UserPlus,
  XCircle,
  type Icon,
} from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { Badge } from "@/components/ui/badge";
import { api } from "@/lib/api/browser";
import { newIdempotencyKey } from "@/lib/api/idempotency";
import { unwrap } from "@/lib/api/unwrap";
import type { NotificationKind } from "@/lib/notification/inbox";

// A Record over the generated enum: a kind the API adds fails the typecheck here until it has an
// icon. The icons are plain ink, because green and red mean coins in this app.
const ICONS: Record<NotificationKind, Icon> = {
  member_joined: UserPlus,
  terms_changed: PencilSimpleLine,
  terms_signed: PencilSimpleLine,
  pact_scheduled: CalendarCheck,
  pact_settled: Receipt,
  proof_submitted: FileText,
  proof_edited: FileText,
  proof_approved: CheckCircle,
  proof_rejected: XCircle,
  proof_auto_approved: Timer,
  proof_overridden: ArrowUUpLeft,
  rejection_final: Prohibit,
  rest_declared: Moon,
  day_missed: CalendarX,
  dispute_opened: Scales,
  dispute_upheld: Scales,
  dispute_dismissed: Scales,
  reminder_cutoff_3h: Alarm,
  reminder_cutoff_30m: Alarm,
  review_deadline_soon: HourglassMedium,
  payout_marked_paid: HandCoins,
  payout_confirmed: SealCheck,
};

/** One notification as the page prepared it: copy and times are already in the visitor's language and time zone. */
export type InboxRow = { id: number; kind: NotificationKind; text: string; pact: string; when: string; href: string | null; unread: boolean };

/**
 * The inbox. Opening it marks what is on the page as read, once, and refreshes the route so the bell
 * drops its count; the "Baru" marks stay for this visit, so the person can still see what was new.
 */
export function NotificationList({ rows }: { rows: InboxRow[] }) {
  const t = useTranslations("Notifications");
  const router = useRouter();
  const [fresh] = useState(() => new Set(rows.filter((r) => r.unread).map((r) => r.id)));
  const sent = useRef(false);

  useEffect(() => {
    if (sent.current || fresh.size === 0) return;
    sent.current = true;
    unwrap(api.POST("/notifications/read", { params: { header: { "Idempotency-Key": newIdempotencyKey() } }, body: { ids: [...fresh] } }))
      .then(() => router.refresh())
      // Nothing to tell the person: the marks simply come back unread on the next visit.
      .catch(() => {});
  }, [fresh, router]);

  return (
    <ul aria-label={t("list")} className="mt-6 max-w-3xl divide-y divide-rule border-y border-rule" data-testid="notifications">
      {rows.map((r) => {
        const isNew = fresh.has(r.id);
        const Glyph = ICONS[r.kind];
        const body = (
          <>
            <Glyph aria-hidden weight="bold" className="mt-0.5 size-5 shrink-0 text-muted" />
            <span className="min-w-0 flex-1">
              <span className={isNew ? "block text-[15px] font-semibold [overflow-wrap:anywhere]" : "block text-[15px] [overflow-wrap:anywhere]"}>{r.text}</span>
              <span className="mt-1 block text-sm text-muted [overflow-wrap:anywhere]">
                {r.pact}
                {" · "}
                <span className="font-mono tabular-nums">{r.when}</span>
              </span>
            </span>
            {isNew ? <Badge tone="cover">{t("unread")}</Badge> : null}
          </>
        );
        const cls = "flex min-h-16 items-start gap-3 py-3 sm:px-2";
        return (
          <li key={r.id} data-kind={r.kind} data-unread={isNew}>
            {r.href ? (
              <Link href={r.href} className={`${cls} outline-offset-4 hover:bg-sunken focus-visible:outline-2 focus-visible:outline-ring`}>
                {body}
              </Link>
            ) : (
              <div className={cls}>{body}</div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
