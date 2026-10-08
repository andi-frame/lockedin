import { useTranslations } from "next-intl";
import type { components } from "@/lib/api/schema";
import { Badge, type BadgeProps } from "@/components/ui/badge";

type PactStatus = components["schemas"]["PactStatus"];

// A Record over the generated enum, so a new status in openapi.yaml fails the typecheck until it
// has a tone. Quiet means "not live yet"; cover means "in motion"; credit means "finished well".
const tones: Record<PactStatus, NonNullable<BadgeProps["tone"]>> = {
  draft: "quiet",
  proposed: "neutral",
  scheduled: "cover",
  active: "cover",
  settling: "neutral",
  completed: "credit",
  cancelled: "quiet",
};

export function PactStatusBadge({ status }: { status: PactStatus }) {
  const t = useTranslations("PactStatus");
  return <Badge tone={tones[status]}>{t(status)}</Badge>;
}
