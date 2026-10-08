import { WarningCircle } from "@phosphor-icons/react/dist/ssr";

/** The message for a failure that belongs to the whole form, not to one field. */
export function FormError({ children }: { children: string }) {
  return (
    <p
      role="alert"
      className="flex items-start gap-2 rounded-control border border-debit bg-debit-tint px-3 py-2.5 text-sm font-medium text-debit"
    >
      <WarningCircle aria-hidden weight="bold" className="mt-0.5 size-4 shrink-0" />
      <span>{children}</span>
    </p>
  );
}

/** Focus the first field that failed, so a keyboard or screen-reader user lands on the problem. */
export function focusFirstInvalid(form: HTMLFormElement, names: readonly string[]) {
  for (const name of names) {
    const el = form.elements.namedItem(name);
    if (el instanceof HTMLElement) {
      el.focus();
      return;
    }
  }
}
