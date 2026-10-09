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
import { formErrorFromApi, validateLogin, type FormError as FormErrorState } from "@/lib/auth/forms";
import { FormError, focusFirstInvalid } from "./form-error";
import { PasswordInput } from "./password-input";
import { useFocusOnDesktop } from "@/lib/use-focus-on-desktop";

const none: FormErrorState = { fields: {} };

export function LoginForm({ next }: { next: string }) {
  const t = useTranslations();
  const router = useRouter();
  const queryClient = useQueryClient();
  const [error, setError] = useState<FormErrorState>(none);
  const [pending, setPending] = useState(false);

  const emailRef = useFocusOnDesktop<HTMLInputElement>();
  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const form = event.currentTarget;
    const data = new FormData(form);
    const checked = validateLogin({ email: String(data.get("email") ?? ""), password: String(data.get("password") ?? "") });
    if (!checked.ok) {
      setError({ fields: checked.errors });
      focusFirstInvalid(form, Object.keys(checked.errors));
      return;
    }
    setError(none);
    setPending(true);
    try {
      await unwrap(api.POST("/auth/login", { body: checked.value }));
      // Whatever was cached belonged to nobody or to the previous account.
      queryClient.clear();
      router.replace(next);
      router.refresh();
    } catch (err) {
      const mapped = formErrorFromApi(err);
      setError(mapped);
      setPending(false);
      focusFirstInvalid(form, Object.keys(mapped.fields));
    }
  }

  return (
    <form onSubmit={onSubmit} noValidate className="flex flex-col gap-5">
      <Field label={t("Auth.emailLabel")} error={error.fields.email ? t(error.fields.email) : undefined}>
        <Input name="email" type="email" inputMode="email" autoComplete="email" autoCapitalize="none" spellCheck={false} ref={emailRef} />
      </Field>
      <Field label={t("Auth.passwordLabel")} error={error.fields.password ? t(error.fields.password) : undefined}>
        <PasswordInput name="password" autoComplete="current-password" />
      </Field>
      {error.form ? <FormError>{t(error.form, error.params)}</FormError> : null}
      <Button type="submit" block loading={pending}>
        {pending ? t("Auth.working") : t("Auth.login")}
      </Button>
    </form>
  );
}
