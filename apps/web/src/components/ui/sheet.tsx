"use client";

import { Dialog as D } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";
import { CloseButton, DialogDescription, DialogTitle, overlayClasses } from "./dialog";

export const Sheet = D.Root;
export const SheetTrigger = D.Trigger;
export const SheetClose = D.Close;
export const SheetTitle = DialogTitle;
export const SheetDescription = DialogDescription;

const sides = {
  // Phones: a bottom sheet sits under the thumb. Wide screens: a right-hand panel.
  bottom: [
    "inset-x-0 bottom-0 max-h-[85dvh] rounded-t-panel border-t",
    "data-[state=open]:motion-safe:animate-sheet-in-bottom data-[state=closed]:motion-safe:animate-sheet-out-bottom",
  ],
  right: [
    "inset-y-0 right-0 h-full w-[min(26rem,100vw-2rem)] border-l",
    "data-[state=open]:motion-safe:animate-sheet-in-right data-[state=closed]:motion-safe:animate-sheet-out-right",
  ],
} as const;

export function SheetContent({
  side = "bottom",
  className,
  children,
  ...props
}: ComponentProps<typeof D.Content> & { side?: keyof typeof sides }) {
  return (
    <D.Portal>
      <D.Overlay className={overlayClasses} />
      <D.Content
        className={cn(
          "fixed z-50 flex flex-col gap-4 overflow-y-auto overscroll-contain border-rule-strong bg-surface p-5 pb-[max(1.25rem,env(safe-area-inset-bottom))] pr-14 text-ink shadow-overlay",
          sides[side],
          className,
        )}
        {...props}
      >
        {children}
        <CloseButton />
      </D.Content>
    </D.Portal>
  );
}
