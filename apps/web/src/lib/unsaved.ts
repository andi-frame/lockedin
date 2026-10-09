"use client";

import { useEffect } from "react";

/**
 * The listener behind the browser's "leave this page?" prompt. It reads `active` when the page is
 * left, so one listener serves every render. A hard exit (closing the tab, reloading, another
 * address) is all the browser lets a page guard; moving inside the app is not covered.
 */
export function beforeUnloadHandler(active: () => boolean) {
  return (e: { preventDefault(): void; returnValue?: unknown }) => {
    if (!active()) return;
    e.preventDefault();
    e.returnValue = "";
  };
}

/** Asks the browser to confirm before the page is closed or reloaded while `dirty` is true. */
export function useUnsavedChangesWarning(dirty: boolean) {
  useEffect(() => {
    if (!dirty) return;
    const handler = beforeUnloadHandler(() => true);
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [dirty]);
}
