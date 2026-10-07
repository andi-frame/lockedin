"use client";

import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";
import { useFieldControl } from "./field";

export const controlClasses = [
  "w-full rounded-control border border-rule-strong bg-surface px-3 text-[15px] text-ink",
  "placeholder:text-placeholder transition-colors duration-150",
  "hover:border-ink/60 focus-visible:border-ring focus-visible:outline-2 focus-visible:outline-offset-0 focus-visible:outline-ring",
  "aria-invalid:border-debit aria-invalid:focus-visible:outline-debit",
  "disabled:cursor-not-allowed disabled:border-rule disabled:bg-sunken disabled:text-muted",
].join(" ");

export function Input({ className, ...props }: ComponentProps<"input">) {
  return <input {...useFieldControl()} className={cn(controlClasses, "h-11", className)} {...props} />;
}

export function Textarea({ className, ...props }: ComponentProps<"textarea">) {
  return (
    <textarea {...useFieldControl()} className={cn(controlClasses, "min-h-28 resize-y py-2.5 leading-6", className)} {...props} />
  );
}
