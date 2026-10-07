"use client";

import { X } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import { Dialog as D } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";
import { Button } from "./button";

export const Dialog = D.Root;
export const DialogTrigger = D.Trigger;
export const DialogClose = D.Close;

export function DialogTitle({ className, ...props }: ComponentProps<typeof D.Title>) {
  return <D.Title className={cn("text-lg font-semibold leading-snug tracking-tight text-ink", className)} {...props} />;
}

export function DialogDescription({ className, ...props }: ComponentProps<typeof D.Description>) {
  return <D.Description className={cn("text-[15px] text-muted", className)} {...props} />;
}

export const overlayClasses =
  "fixed inset-0 z-40 bg-scrim data-[state=open]:motion-safe:animate-overlay-in data-[state=closed]:motion-safe:animate-overlay-out";

export function CloseButton() {
  const t = useTranslations("Common");
  return (
    <D.Close asChild>
      <Button variant="ghost" size="icon-sm" aria-label={t("close")} className="absolute right-3 top-3">
        <X aria-hidden weight="bold" className="size-4" />
      </Button>
    </D.Close>
  );
}

// A modal is for a decision that needs protected focus (confirm a decision, sign terms). Anything
// else belongs on the page.
export function DialogContent({ className, children, ...props }: ComponentProps<typeof D.Content>) {
  return (
    <D.Portal>
      <D.Overlay className={overlayClasses} />
      <D.Content
        className={cn(
          "fixed left-1/2 top-1/2 z-50 flex max-h-[calc(100dvh-2rem)] w-[calc(100vw-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 flex-col gap-4 overflow-y-auto",
          "rounded-panel border border-rule-strong bg-surface p-5 pr-14 text-ink shadow-overlay",
          "data-[state=open]:motion-safe:animate-dialog-in data-[state=closed]:motion-safe:animate-dialog-out",
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

export function DialogHeader({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("flex flex-col gap-1.5", className)} {...props} />;
}

export function DialogFooter({ className, ...props }: ComponentProps<"div">) {
  return <div className={cn("flex flex-col-reverse gap-2 sm:flex-row sm:justify-end", className)} {...props} />;
}
