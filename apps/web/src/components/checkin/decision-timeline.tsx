import {
  CalendarX,
  CheckCircle,
  Clock,
  Gavel,
  Lock,
  Moon,
  PaperPlaneTilt,
  Scales,
  SealCheck,
  XCircle,
} from "@phosphor-icons/react/dist/ssr";
import { useFormatter, useTranslations } from "next-intl";
import type { ComponentType } from "react";
import { cn } from "@/lib/cn";
import type { components } from "@/lib/api/schema";
import { timelineEntries, type TimelineEntry } from "@/lib/checkin/timeline";

type Decision = components["schemas"]["Decision"];

const icons: Record<Decision["action"], ComponentType<{ "aria-hidden"?: boolean; weight?: "bold"; className?: string }>> = {
  submit: PaperPlaneTilt,
  approve: SealCheck,
  reject: XCircle,
  auto_approve: CheckCircle,
  override: Gavel,
  dispute: Scales,
  uphold: Gavel,
  dismiss: Gavel,
  dispute_expired: Clock,
  rest: Moon,
  missed: CalendarX,
  finalize: Lock,
};

/**
 * Every decision on this check-in, oldest first. The backer's extra reach (override, uphold,
 * dismiss) is drawn in stamp violet and labelled, so power is never silent (SPEC §2); the decisions
 * people take about a proof wear the stamp on their icon; the worker's steps stay quiet.
 */
export function DecisionTimeline({
  decisions,
  me,
  names,
  timeZone,
}: {
  decisions: Decision[];
  me: string;
  names: Record<string, string>;
  timeZone: string;
}) {
  const t = useTranslations();
  const f = useFormatter();
  const entries = timelineEntries(decisions, { me, names });
  const actionOf = new Map(decisions.map((d) => [d.id, d.action]));
  const who = (e: TimelineEntry) => (e.who === "self" ? t("Detail.you") : e.who === "system" ? t("Detail.system") : e.name);

  if (entries.length === 0) return <p className="text-[15px] text-muted">{t("Detail.timelineEmpty")}</p>;
  return (
    <ol className="divide-y divide-rule border-y border-rule" data-testid="timeline">
      {entries.map((e) => {
        const Icon = icons[actionOf.get(e.id) ?? "submit"];
        return (
          <li
            key={e.id}
            data-tone={e.tone}
            className={cn("flex gap-3 py-3", e.tone === "power" && "-mx-3 border-0 bg-stamp-tint px-3 text-stamp-text")}
          >
            <Icon
              aria-hidden
              weight="bold"
              className={cn("mt-0.5 size-5 shrink-0", e.tone === "power" || e.tone === "human" ? "text-stamp-text" : "text-muted")}
            />
            <div className="min-w-0 flex-1">
              <p className="text-[15px] font-medium">{t(e.key, { who: who(e) })}</p>
              {e.tone === "power" ? <p className="mt-0.5 text-[13px] font-semibold">{t("Detail.power")}</p> : null}
              {e.reason ? (
                <p className="mt-1 text-[15px] [overflow-wrap:anywhere]">
                  <span className="text-[13px] text-muted">{t("Detail.reason")}: </span>
                  {e.reason}
                </p>
              ) : null}
              <p className="mt-1 font-mono text-[13px] tabular-nums text-muted">{f.dateTime(new Date(e.at), { dateStyle: "medium", timeStyle: "short", timeZone })}</p>
            </div>
          </li>
        );
      })}
    </ol>
  );
}
