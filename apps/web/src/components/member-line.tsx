import { useTranslations } from "next-intl";
import { cn } from "@/lib/cn";
import type { MemberSlot } from "@/lib/member";

// Static class strings (Tailwind cannot see a class built from a variable).
const slots = {
  0: { mark: "bg-member-a text-member-a-foreground", line: "decoration-member-a" },
  1: { mark: "bg-member-b text-member-b-foreground", line: "decoration-member-b" },
} as const satisfies Record<MemberSlot, { mark: string; line: string }>;

function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  const first = parts[0]?.[0] ?? "?";
  const last = parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? "") : "";
  return (first + last).toUpperCase();
}

/**
 * A member named in their own fixed line colour: a monogram plus the name, underlined with a
 * ledger rule. The name is always printed, so the colour only helps and never carries meaning alone.
 */
export function MemberLine({
  name,
  slot,
  role,
  className,
}: {
  name: string;
  slot: MemberSlot;
  role?: "backer" | "doer";
  className?: string;
}) {
  const t = useTranslations("Role");
  return (
    <span className={cn("inline-flex items-center gap-2", className)}>
      <span
        aria-hidden
        className={cn(
          "inline-flex size-7 shrink-0 items-center justify-center rounded-control font-mono text-xs font-semibold",
          slots[slot].mark,
        )}
      >
        {initials(name)}
      </span>
      <span className="min-w-0">
        <span
          className={cn(
            "block truncate font-medium underline decoration-2 underline-offset-4",
            slots[slot].line,
          )}
        >
          {name}
        </span>
        {role ? <span className="block text-[13px] leading-tight text-muted">{t(role)}</span> : null}
      </span>
    </span>
  );
}
