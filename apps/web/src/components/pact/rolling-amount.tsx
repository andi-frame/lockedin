"use client";

import { useEffect, useRef, useState, type ComponentProps } from "react";
import { Amount } from "@/components/amount";
import { rollValue } from "@/lib/passbook";

const ROLL_MS = 480;

/**
 * The saldo, rolling from its old figure to the new one after a line is printed (the second half of
 * the signature move). `coins` is always the server's balance; this only shows the way there. Under
 * reduced motion it lands at once.
 */
export function RollingAmount({ coins, delayMs = 0, ...props }: ComponentProps<typeof Amount> & { delayMs?: number }) {
  const [shown, setShown] = useState(coins);
  const current = useRef(coins);

  useEffect(() => {
    const from = current.current;
    if (from === coins) return;
    const instant = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    const begin = performance.now() + delayMs;
    let frame = 0;
    const step = (now: number) => {
      const progress = instant ? 1 : (now - begin) / ROLL_MS;
      const value = progress <= 0 ? from : rollValue(from, coins, progress);
      current.current = value;
      setShown(value);
      if (progress < 1) frame = requestAnimationFrame(step);
    };
    frame = requestAnimationFrame(step);
    return () => cancelAnimationFrame(frame);
  }, [coins, delayMs]);

  return <Amount coins={shown} {...props} />;
}
