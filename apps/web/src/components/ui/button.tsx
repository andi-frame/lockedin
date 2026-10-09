import { CircleNotch } from "@phosphor-icons/react/dist/ssr";
import { cva, type VariantProps } from "class-variance-authority";
import { Slot } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";

// `decision` and `decision-quiet` are the stamp-violet buttons. They exist for human decisions
// only (approve, reject, override, sign terms); everything else uses primary, secondary or ghost.
export const buttonVariants = cva(
  [
    "inline-flex shrink-0 select-none items-center justify-center gap-2 whitespace-nowrap rounded-control font-medium",
    "transition-[background-color,color,border-color,transform] duration-150 ease-out",
    "active:translate-y-px disabled:pointer-events-none disabled:opacity-50 aria-busy:pointer-events-none",
    "[&_svg]:shrink-0",
  ],
  {
    variants: {
      variant: {
        primary: "bg-primary text-primary-foreground hover:bg-primary-hover",
        secondary: "border border-rule-strong bg-surface text-ink hover:bg-sunken",
        ghost: "text-ink hover:bg-sunken",
        decision: "bg-stamp text-stamp-foreground hover:bg-stamp-hover",
        "decision-quiet": "border border-stamp-text bg-surface text-stamp-text hover:bg-stamp-tint",
        // On the cover-teal band (the landing page), where the teal primary would vanish.
        onCover: "bg-cover-ink text-cover hover:bg-cover-ink/90",
        onCoverQuiet: "border border-cover-ink/45 text-cover-ink hover:bg-cover-ink/10",
      },
      size: {
        md: "h-11 px-4 text-[15px]",
        sm: "h-9 px-3 text-sm",
        icon: "size-11",
        "icon-sm": "size-9",
      },
      block: { true: "w-full", false: "" },
    },
    defaultVariants: { variant: "primary", size: "md", block: false },
  },
);

export type ButtonProps = ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean;
    /** Keeps the label, swaps the icon slot for a spinner, and blocks further presses. */
    loading?: boolean;
  };

export function Button({ className, variant, size, block, asChild, loading, children, disabled, ...props }: ButtonProps) {
  const classes = cn(buttonVariants({ variant, size, block }), className);
  if (asChild) {
    return (
      <Slot.Root className={classes} {...props}>
        {children}
      </Slot.Root>
    );
  }
  return (
    <button
      type="button"
      className={classes}
      disabled={disabled}
      aria-busy={loading || undefined}
      {...props}
    >
      {loading ? <CircleNotch aria-hidden weight="bold" className="size-4 motion-safe:animate-spin" /> : null}
      {children}
    </button>
  );
}
