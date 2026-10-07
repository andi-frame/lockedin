"use client";

import { Tooltip as T } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";

export function TooltipProvider({ delayDuration = 250, ...props }: ComponentProps<typeof T.Provider>) {
  return <T.Provider delayDuration={delayDuration} {...props} />;
}
export const Tooltip = T.Root;
export const TooltipTrigger = T.Trigger;

export function TooltipContent({ className, sideOffset = 6, ...props }: ComponentProps<typeof T.Content>) {
  return (
    <T.Portal>
      <T.Content
        sideOffset={sideOffset}
        className={cn(
          "z-50 max-w-64 rounded-control bg-ink px-2.5 py-1.5 text-[13px] leading-5 text-ground shadow-overlay motion-safe:animate-pop-in",
          className,
        )}
        {...props}
      />
    </T.Portal>
  );
}
