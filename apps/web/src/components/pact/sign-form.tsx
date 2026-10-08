"use client";

import { Signature } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import { useState, type FormEvent } from "react";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { checkSignature } from "@/lib/pact/agreement";

/**
 * Signing is a human decision, so the button is stamp violet (SPEC §4): tick that you read the
 * summary above, type your display name, and sign. `onSign` does the API work and throws a message
 * key (under `Errors`) on failure.
 */
export function SignForm({
  displayName,
  label,
  onSign,
}: {
  displayName: string;
  label: string;
  onSign: (typedName: string) => Promise<void>;
}) {
  const t = useTranslations();
  const [agreed, setAgreed] = useState(false);
  const [typed, setTyped] = useState("");
  const [errors, setErrors] = useState<{ agreed?: string; name?: string }>({});
  const [formError, setFormError] = useState<string>();
  const [pending, setPending] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const checked = checkSignature({ agreed, typed, displayName });
    if (!checked.ok) {
      setErrors(checked.errors);
      const first = checked.errors.agreed ? "agreed" : "name";
      (event.currentTarget.elements.namedItem(first) as HTMLElement | null)?.focus();
      return;
    }
    setErrors({});
    setFormError(undefined);
    setPending(true);
    try {
      await onSign(typed.trim());
    } catch (err) {
      setFormError(err instanceof Error ? err.message : t("Errors.unknown"));
      setPending(false);
    }
  }

  return (
    <form onSubmit={submit} noValidate className="flex max-w-xl flex-col gap-5">
      <div className="flex flex-col gap-2">
        <label className="flex min-h-11 cursor-pointer items-start gap-3 text-[15px]">
          <input
            type="checkbox"
            name="agreed"
            checked={agreed}
            aria-invalid={errors.agreed ? true : undefined}
            onChange={(ev) => setAgreed(ev.target.checked)}
            className="mt-1 size-5 shrink-0 accent-stamp"
          />
          <span className="font-medium">{t("Sign.agreeLabel")}</span>
        </label>
        {errors.agreed ? (
          <p role="alert" className="text-sm font-medium text-debit">
            {t(errors.agreed)}
          </p>
        ) : null}
      </div>
      <Field label={t("Sign.nameLabel")} hint={t("Sign.nameHint", { name: displayName })} error={errors.name ? t(errors.name) : undefined}>
        <Input name="name" value={typed} autoComplete="off" autoCapitalize="words" spellCheck={false} onChange={(ev) => setTyped(ev.target.value)} />
      </Field>
      {formError ? <FormError>{formError}</FormError> : null}
      <div>
        <Button type="submit" variant="decision" loading={pending}>
          <Signature aria-hidden weight="bold" className="size-4" />
          {pending ? t("Sign.signing") : label}
        </Button>
      </div>
    </form>
  );
}
