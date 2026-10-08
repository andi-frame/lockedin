"use client";

import { useTranslations } from "next-intl";
import { cn } from "@/lib/cn";

const days = [1, 2, 3, 4, 5, 6, 7] as const;

/** Seven toggles for ISO weekdays (1 = Monday). Each has a printed name, so state never rests on colour. */
export function WeekdayPicker({
  name,
  legend,
  value,
  onChange,
  error,
}: {
  name: string;
  legend: string;
  value: number[];
  onChange: (next: number[]) => void;
  error?: string;
}) {
  const t = useTranslations("Weekday");
  const toggle = (d: number) => onChange(value.includes(d) ? value.filter((x) => x !== d) : [...value, d].sort((a, b) => a - b));
  return (
    <fieldset className="flex min-w-0 flex-col gap-2">
      <legend className="mb-2 text-sm font-medium text-ink">{legend}</legend>
      <div className="flex flex-wrap gap-2" role="group" aria-label={legend}>
        {days.map((d, i) => {
          const on = value.includes(d);
          return (
            <button
              key={d}
              type="button"
              // The first button carries the field name so a failed check can move focus here.
              name={i === 0 ? name : undefined}
              aria-pressed={on}
              onClick={() => toggle(d)}
              className={cn(
                "h-11 min-w-12 rounded-control border px-2 text-sm font-medium transition-colors duration-150",
                "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
                on
                  ? "border-primary bg-primary text-primary-foreground"
                  : "border-rule-strong bg-surface text-ink hover:bg-sunken",
                error && !on && "border-debit",
              )}
            >
              {t(String(d))}
            </button>
          );
        })}
      </div>
      {error ? (
        <p role="alert" className="text-sm font-medium text-debit">
          {error}
        </p>
      ) : null}
    </fieldset>
  );
}
