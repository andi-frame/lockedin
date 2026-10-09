"use client";

import { useEffect, useRef, type ReactNode } from "react";

/**
 * The signature move for the landing page's example month: its stamps and printed lines come in
 * once, when it scrolls into view. Without script, or with reduced motion, nothing is hidden: the
 * attribute is only set after the page has loaded, and only when the animation will really play.
 * The state lives on the element (`data-print`: armed, then done) so no re-render is needed.
 */
export function PrintOnView({ children, className }: { children: ReactNode; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = ref.current;
    if (!el || window.matchMedia("(prefers-reduced-motion: reduce)").matches || !("IntersectionObserver" in window)) return;
    el.dataset.print = "armed";
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry?.isIntersecting) {
          el.dataset.print = "done";
          observer.disconnect();
        }
      },
      { threshold: 0.2 },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  return (
    <div ref={ref} className={className}>
      {children}
    </div>
  );
}
