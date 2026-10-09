"use client";

import { useEffect, useRef } from "react";

/**
 * Focuses the field when the page opens, but only with a mouse (`pointer: fine`). On a phone the
 * same focus raises the keyboard over the page before the person has seen it.
 */
export function useFocusOnDesktop<T extends HTMLElement>() {
  const ref = useRef<T>(null);
  useEffect(() => {
    if (window.matchMedia("(pointer: fine)").matches) ref.current?.focus();
  }, []);
  return ref;
}
