import { clsx, type ClassValue } from "clsx";
import { extendTailwindMerge } from "tailwind-merge";

// Teach tailwind-merge our token names, otherwise `rounded-control` + `rounded-panel` or
// `text-ink` + `text-muted` would not be recognised as conflicting.
const merge = extendTailwindMerge({
  extend: {
    theme: {
      radius: ["control", "panel"],
      shadow: ["panel", "overlay"],
      color: [
        "ground", "surface", "sunken", "rule", "rule-strong", "ink", "muted", "placeholder",
        "cover", "cover-ink", "cover-muted", "primary", "primary-hover", "primary-foreground",
        "teal-tint", "teal-text", "debit", "debit-tint", "credit", "credit-tint",
        "stamp", "stamp-hover", "stamp-foreground", "stamp-tint", "stamp-text",
        "today", "today-ink", "member-a", "member-a-foreground", "member-b", "member-b-foreground", "ring", "scrim",
      ],
    },
  },
});

export function cn(...inputs: ClassValue[]): string {
  return merge(clsx(inputs));
}
