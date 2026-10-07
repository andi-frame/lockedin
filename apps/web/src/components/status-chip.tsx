import {
  CalendarX,
  CheckCircle,
  CircleDashed,
  Moon,
  PaperPlaneTilt,
  Scales,
  SealCheck,
  XCircle,
} from "@phosphor-icons/react/dist/ssr";
import { useTranslations } from "next-intl";
import type { ComponentType } from "react";
import type { components } from "@/lib/api/schema";
import { Badge, type BadgeProps } from "./ui/badge";

export type CheckInStatus = components["schemas"]["CheckInStatus"];

type Spec = {
  tone: NonNullable<BadgeProps["tone"]>;
  Icon: ComponentType<{ "aria-hidden"?: boolean; weight?: "regular" | "bold" | "fill" }>;
};

// A Record over the generated enum: adding a status to openapi.yaml fails the typecheck here until
// it has a chip. Every status has its own icon and label, so colour is never the only signal.
// approved and rejected wear the stamp tone because they are the outcome of a human decision.
const specs: Record<CheckInStatus, Spec> = {
  open: { tone: "neutral", Icon: CircleDashed },
  submitted: { tone: "cover", Icon: PaperPlaneTilt },
  approved: { tone: "stamp", Icon: SealCheck },
  auto_approved: { tone: "cover", Icon: CheckCircle },
  rejected: { tone: "stamp", Icon: XCircle },
  disputed: { tone: "neutral", Icon: Scales },
  missed: { tone: "debit", Icon: CalendarX },
  rest: { tone: "quiet", Icon: Moon },
};

export const checkInStatuses = Object.keys(specs) as CheckInStatus[];

export function StatusChip({ status, className }: { status: CheckInStatus; className?: string }) {
  const t = useTranslations("Status");
  const { tone, Icon } = specs[status];
  return (
    <Badge tone={tone} className={className} data-status={status}>
      <Icon aria-hidden weight="bold" />
      {t(status)}
    </Badge>
  );
}
