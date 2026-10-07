"use client";

import { CaretDown, Check } from "@phosphor-icons/react";
import { Select as S } from "radix-ui";
import type { ComponentProps } from "react";
import { cn } from "@/lib/cn";
import { useFieldControl } from "./field";
import { controlClasses } from "./input";

export const Select = S.Root;
export const SelectGroup = S.Group;
export const SelectValue = S.Value;

export function SelectTrigger({ className, children, ...props }: ComponentProps<typeof S.Trigger>) {
  return (
    <S.Trigger
      {...useFieldControl()}
      className={cn(
        controlClasses,
        "flex h-11 items-center justify-between gap-2 text-left data-[placeholder]:text-placeholder",
        className,
      )}
      {...props}
    >
      <span className="min-w-0 truncate">{children}</span>
      <S.Icon>
        <CaretDown aria-hidden weight="bold" className="size-4 text-muted" />
      </S.Icon>
    </S.Trigger>
  );
}

export function SelectContent({ className, children, ...props }: ComponentProps<typeof S.Content>) {
  return (
    <S.Portal>
      <S.Content
        position="popper"
        sideOffset={4}
        className={cn(
          "z-50 max-h-[min(18rem,var(--radix-select-content-available-height))] min-w-[var(--radix-select-trigger-width)] overflow-hidden",
          "rounded-panel border border-rule-strong bg-surface text-ink shadow-overlay motion-safe:animate-pop-in",
          className,
        )}
        {...props}
      >
        <S.Viewport className="p-1">{children}</S.Viewport>
      </S.Content>
    </S.Portal>
  );
}

export function SelectItem({ className, children, ...props }: ComponentProps<typeof S.Item>) {
  return (
    <S.Item
      className={cn(
        "relative flex min-h-10 cursor-pointer select-none items-center rounded-control py-2 pl-8 pr-3 text-[15px] outline-none",
        "data-[highlighted]:bg-sunken data-[disabled]:pointer-events-none data-[disabled]:opacity-50",
        className,
      )}
      {...props}
    >
      <span className="absolute left-2 inline-flex size-4 items-center justify-center">
        <S.ItemIndicator>
          <Check aria-hidden weight="bold" className="size-4 text-teal-text" />
        </S.ItemIndicator>
      </span>
      <S.ItemText>{children}</S.ItemText>
    </S.Item>
  );
}
