"use client";

import { useTranslations } from "next-intl";
import { useState, type FormEvent } from "react";
import { FormError } from "@/components/auth/form-error";
import { PasswordInput } from "@/components/auth/password-input";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { toast } from "@/components/ui/toast";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import { unwrap } from "@/lib/api/unwrap";
import { validatePasswordChange } from "@/lib/settings/password";

// Already translated: client checks and the server's answers both end up as text for the field.
type FieldText = Partial<Record<"current" | "next", string>>;

/**
 * Changing the password needs the current one. The server ends every other session and keeps this
 * one, so the person stays here; the toast says what happened to the others.
 */
export function PasswordForm() {
  const t = useTranslations("Settings");
  const tAll = useTranslations();
  const [errors, setErrors] = useState<FieldText>({});
  const [formError, setFormError] = useState<string>();
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    const input = { current: String(data.get("current") ?? ""), next: String(data.get("next") ?? "") };
    const checked = validatePasswordChange(input);
    if (!checked.ok) {
      setErrors({
        current: checked.errors.current ? tAll(checked.errors.current) : undefined,
        next: checked.errors.next ? tAll(checked.errors.next) : undefined,
      });
      setFormError(undefined);
      (form.elements.namedItem(checked.errors.current ? "current" : "next") as HTMLElement | null)?.focus();
      return;
    }
    setErrors({});
    setFormError(undefined);
    setPending(true);
    try {
      await unwrap(api.POST("/me/password", { body: { current_password: input.current, new_password: input.next } }));
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "unknown";
      const message = tAll(`Errors.${errorMessageKey(code)}`);
      if (code === "auth.wrong_password") setErrors({ current: message });
      else if (code === "auth.weak_password") setErrors({ next: message });
      else setFormError(message);
      setPending(false);
      return;
    }
    form.reset();
    setPending(false);
    toast({ title: t("passwordChanged"), description: t("passwordChangedBody"), tone: "success" });
  }

  return (
    <form onSubmit={onSubmit} noValidate className="mt-3 flex flex-col gap-5">
      <Field label={t("currentPassword")} error={errors.current}>
        <PasswordInput name="current" autoComplete="current-password" />
      </Field>
      <Field label={t("newPassword")} hint={t("newPasswordHint")} error={errors.next}>
        <PasswordInput name="next" autoComplete="new-password" />
      </Field>
      {formError ? <FormError>{formError}</FormError> : null}
      <div>
        <Button type="submit" loading={pending}>
          {pending ? t("saving") : t("changePassword")}
        </Button>
      </div>
    </form>
  );
}
