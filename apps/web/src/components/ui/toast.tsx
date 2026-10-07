"use client";

import { CheckCircle, Info, WarningCircle, X } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import { Toast as T } from "radix-ui";
import { useSyncExternalStore, type ComponentType } from "react";
import { cn } from "@/lib/cn";
import { Button } from "./button";

type Tone = "neutral" | "success" | "error";
type ToastItem = { id: number; title: string; description?: string; tone: Tone };

// A tiny external store so any client code can call toast() without a provider hook. Toasts are
// for transient confirmation; a problem the user must act on stays inline next to its field.
let items: ToastItem[] = [];
let nextId = 1;
const listeners = new Set<() => void>();
const emit = () => listeners.forEach((l) => l());

export function toast(input: { title: string; description?: string; tone?: Tone }): number {
  const id = nextId++;
  items = [...items, { id, title: input.title, description: input.description, tone: input.tone ?? "neutral" }];
  emit();
  return id;
}

export function dismissToast(id: number): void {
  items = items.filter((i) => i.id !== id);
  emit();
}

const subscribe = (l: () => void) => {
  listeners.add(l);
  return () => void listeners.delete(l);
};
const none: ToastItem[] = [];

const tones: Record<Tone, { Icon: ComponentType<{ className?: string; "aria-hidden"?: boolean; weight?: "fill" | "bold" }>; icon: string }> = {
  neutral: { Icon: Info, icon: "text-ink" },
  success: { Icon: CheckCircle, icon: "text-teal-text" },
  error: { Icon: WarningCircle, icon: "text-debit" },
};

export function Toaster() {
  const t = useTranslations("Common");
  const list = useSyncExternalStore(subscribe, () => items, () => none);
  return (
    <T.Provider swipeDirection="right" duration={5000}>
      {list.map((item) => {
        const { Icon, icon } = tones[item.tone];
        return (
          <T.Root
            key={item.id}
            // An error interrupts a screen reader; confirmations wait their turn.
            type={item.tone === "error" ? "foreground" : "background"}
            duration={item.tone === "error" ? 8000 : 5000}
            onOpenChange={(open) => {
              if (!open) dismissToast(item.id);
            }}
            className={cn(
              "flex w-full items-start gap-3 rounded-panel border border-rule-strong bg-surface p-3.5 pr-2 text-ink shadow-overlay",
              "data-[state=open]:motion-safe:animate-toast-in data-[state=closed]:motion-safe:animate-toast-out",
            )}
          >
            <Icon aria-hidden weight="fill" className={cn("mt-0.5 size-5 shrink-0", icon)} />
            <div className="min-w-0 flex-1">
              <T.Title className="text-[15px] font-medium leading-snug">{item.title}</T.Title>
              {item.description ? (
                <T.Description className="mt-0.5 text-sm text-muted">{item.description}</T.Description>
              ) : null}
            </div>
            <T.Close asChild>
              <Button variant="ghost" size="icon-sm" aria-label={t("dismissToast")}>
                <X aria-hidden weight="bold" className="size-4" />
              </Button>
            </T.Close>
          </T.Root>
        );
      })}
      <T.Viewport
        className={cn(
          "fixed inset-x-0 bottom-0 z-[60] m-0 flex list-none flex-col gap-2 p-4 pb-[max(1rem,env(safe-area-inset-bottom))] outline-none",
          "sm:inset-x-auto sm:right-0 sm:w-96",
        )}
      />
    </T.Provider>
  );
}
