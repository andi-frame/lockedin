"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api/browser";
import { unwrap } from "@/lib/api/unwrap";
import { formErrorFromApi, validateRegister, type FormError as FormErrorState } from "@/lib/auth/forms";
import { FormError, focusFirstInvalid } from "./form-error";
import { PasswordInput } from "./password-input";

const none: FormErrorState = { fields: {} };

export function RegisterForm({ next, locale }: { next: string; locale: "id" | "en" }) {
  const t = useTranslations();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [error, setError] = useState<FormErrorState>(none);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    const checked = validateRegister({
      name: String(data.get("name") ?? ""),
      email: String(data.get("email") ?? ""),
      password: String(data.get("password") ?? ""),
    });
    if (!checked.ok) {
      setError({ fields: checked.errors });
      // Document order, not the order the checks ran in.
      focusFirstInvalid(form, ["name", "email", "password"].filter((n) => n in checked.errors));
      return;
    }
    setError(none);
    setPending(true);
    try {
      // The timezone is the browser's, so deadlines default to where the person actually studies.
      const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || "Asia/Jakarta";
      await unwrap(api.POST("/auth/register", { body: { ...checked.value, locale, timezone } }));
      queryClient.clear();
      router.replace(next);
      router.refresh();
    } catch (err) {
      const mapped = formErrorFromApi(err);
      setError(mapped);
      setPending(false);
      focusFirstInvalid(form, ["name", "email", "password"].filter((n) => n in mapped.fields));
    }
  }

  return (
    <form onSubmit={onSubmit} noValidate className="flex flex-col gap-5">
      <Field label={t("Auth.nameLabel")} hint={t("Auth.nameHint")} error={error.fields.name ? t(error.fields.name) : undefined}>
        <Input name="name" autoComplete="name" autoFocus />
      </Field>
      <Field label={t("Auth.emailLabel")} error={error.fields.email ? t(error.fields.email) : undefined}>
        <Input name="email" type="email" inputMode="email" autoComplete="email" autoCapitalize="none" spellCheck={false} />
      </Field>
      <Field label={t("Auth.passwordLabel")} hint={t("Auth.passwordHint")} error={error.fields.password ? t(error.fields.password) : undefined}>
        <PasswordInput name="password" autoComplete="new-password" />
      </Field>
      {error.form ? <FormError>{t(error.form, error.params)}</FormError> : null}
      <Button type="submit" block loading={pending}>
        {pending ? t("Auth.working") : t("Auth.register")}
      </Button>
    </form>
  );
}
