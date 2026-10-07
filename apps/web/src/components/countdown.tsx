"use client";

import { useTranslations } from "next-intl";
import { useMemo, useSyncExternalStore } from "react";
import { cn } from "@/lib/cn";
import { formatClock, remaining, spokenMinutes } from "@/lib/countdown";

// One shared second-resolution store per component instance. `skew` lines the browser clock up
// with the server's (taken once, when the component subscribes) because the deadline is a server
// decision and a phone clock can be minutes off.
function createClock(serverNowMs: number | null) {
  let skew = 0;
  return {
    subscribe(onTick: () => void) {
      skew = serverNowMs === null ? 0 : serverNowMs - Date.now();
      onTick();
      const id = setInterval(onTick, 1000);
      return () => clearInterval(id);
    },
    getSnapshot: () => Math.floor((Date.now() + skew) / 1000),
    // Server render and hydration agree on this value, so there is no hydration mismatch.
    getServerSnapshot: () => (serverNowMs === null ? null : Math.floor(serverNowMs / 1000)),
  };
}

const sizes = {
  md: "text-xl",
  lg: "text-4xl",
} as const;

/**
 * Time to cutoff in fixed cells (the split-flap board in the contract): every digit sits in its
 * own cell, so the figure never jitters as digits change width. The digits tick every second but
 * are hidden from assistive tech; a separate polite live region speaks once per minute.
 */
export function Countdown({
  until,
  serverNow,
  onCover = false,
  size = "md",
  className,
}: {
  /** The instant the window closes, RFC 3339. Computed by the server. */
  until: string;
  /** The server's clock at render time, RFC 3339. Lets the first paint show real digits. */
  serverNow?: string;
  /** Set on the cover-teal band, which needs light digits. */
  onCover?: boolean;
  size?: keyof typeof sizes;
  className?: string;
}) {
  const t = useTranslations("Countdown");
  const serverNowMs = serverNow ? Date.parse(serverNow) : null;
  const clock = useMemo(() => createClock(serverNowMs), [serverNowMs]);
  const nowSec = useSyncExternalStore(clock.subscribe, clock.getSnapshot, clock.getServerSnapshot);

  const r = nowSec === null ? null : remaining(Date.parse(until), nowSec * 1000);
  const text = r ? formatClock(r) : "--:--:--";
  const minutes = r ? spokenMinutes(r) : null;
  const spoken =
    r === null ? "" : minutes === null ? t("spokenExpired") : t("spoken", { hours: Math.floor(minutes / 60), minutes: minutes % 60 });

  return (
    <span className={cn("inline-flex items-baseline gap-2", onCover ? "text-cover-ink" : "text-ink", className)}>
      <span aria-hidden className={cn("inline-flex items-center font-mono font-medium leading-none", sizes[size])}>
        {[...text].map((ch, i) =>
          ch === ":" ? (
            <span key={i} className="px-[0.12em] pb-[0.1em] opacity-60">
              :
            </span>
          ) : (
            <span
              key={i}
              className={cn(
                "inline-block w-[1.2ch] rounded-control py-[0.18em] text-center",
                onCover ? "bg-cover-ink/10" : "bg-sunken",
              )}
            >
              {ch}
            </span>
          ),
        )}
      </span>
      <span aria-hidden className={cn("text-sm", onCover ? "text-cover-muted" : "text-muted")}>
        {r?.expired ? t("over") : t("left")}
      </span>
      <span className="sr-only" aria-live="polite" aria-atomic="true">
        {spoken}
      </span>
    </span>
  );
}
