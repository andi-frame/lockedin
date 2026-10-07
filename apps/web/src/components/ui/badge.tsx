import { cva, type VariantProps } from "class-variance-authority";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";

// Tone follows meaning, not decoration. `today` (highlighter) marks today and nothing else;
// `stamp` is for the outcome of a human decision.
export const badgeVariants = cva(
  "inline-flex shrink-0 items-center gap-1.5 whitespace-nowrap rounded-control border px-2 py-0.5 text-[13px] font-medium leading-5 [&_svg]:size-3.5 [&_svg]:shrink-0",
  {
    variants: {
      tone: {
        neutral: "border-rule-strong bg-surface text-ink",
        quiet: "border-rule bg-sunken text-muted",
        cover: "border-transparent bg-teal-tint text-teal-text",
        debit: "border-transparent bg-debit-tint text-debit",
        credit: "border-transparent bg-credit-tint text-credit",
        stamp: "border-stamp-text bg-stamp-tint text-stamp-text",
        today: "border-transparent bg-today text-today-ink",
      },
    },
    defaultVariants: { tone: "neutral" },
  },
);

export type BadgeProps = ComponentProps<"span"> & VariantProps<typeof badgeVariants>;

export function Badge({ className, tone, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}
