"use client";

import { createContext, useContext, useId, type ReactNode } from "react";
import { cn } from "@/lib/cn";

type FieldIds = { id: string; hintId: string; errorId: string; hasHint: boolean; hasError: boolean };
const FieldContext = createContext<FieldIds | null>(null);

/** What a control inside a Field needs to wire itself to the label, hint and error. */
export function useFieldControl(): {
  id?: string;
  "aria-describedby"?: string;
  "aria-invalid"?: true;
} {
  const f = useContext(FieldContext);
  if (!f) return {};
  const described = [f.hasHint ? f.hintId : null, f.hasError ? f.errorId : null].filter(Boolean).join(" ");
  return {
    id: f.id,
    "aria-describedby": described || undefined,
    "aria-invalid": f.hasError ? true : undefined,
  };
}

// Label above the control, hint under the label's line, error below the control. A placeholder is
// never the label.
export function Field({
  label,
  hint,
  error,
  children,
  className,
}: {
  label: ReactNode;
  hint?: ReactNode;
  error?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  const base = useId();
  const ids: FieldIds = {
    id: `${base}-control`,
    hintId: `${base}-hint`,
    errorId: `${base}-error`,
    hasHint: Boolean(hint),
    hasError: Boolean(error),
  };
  return (
    <FieldContext.Provider value={ids}>
      <div className={cn("flex flex-col gap-2", className)}>
        <label htmlFor={ids.id} className="text-sm font-medium text-ink">
          {label}
        </label>
        {hint ? (
          <p id={ids.hintId} className="-mt-1 text-sm text-muted">
            {hint}
          </p>
        ) : null}
        {children}
        {error ? (
          <p id={ids.errorId} role="alert" className="text-sm font-medium text-debit">
            {error}
          </p>
        ) : null}
      </div>
    </FieldContext.Provider>
  );
}
