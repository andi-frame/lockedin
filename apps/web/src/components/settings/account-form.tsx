"use client";

import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useMemo, useState, type FormEvent } from "react";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { toast } from "@/components/ui/toast";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import type { components } from "@/lib/api/schema";
import { unwrap } from "@/lib/api/unwrap";
import type { Locale } from "@/i18n/config";
import { setLocaleCookie } from "@/lib/locale-cookie";
import { timezoneChoices } from "@/lib/settings/timezones";

type User = components["schemas"]["User"];

/**
 * Name, language and time zone. Only what changed is sent. The language also sets the cookie the
 * pages read, because the account's language is what emails use and the cookie is what this
 * browser shows; changing one without the other would be a surprise.
 */
export function AccountForm({ user }: { user: Pick<User, "display_name" | "email" | "locale" | "timezone"> }) {
  const t = useTranslations("Settings");
  const tErr = useTranslations("Errors");
  const router = useRouter();
  const [name, setName] = useState(user.display_name);
  const [locale, setLocale] = useState<Locale>(user.locale);
  const [timezone, setTimezone] = useState(user.timezone);
  const [nameError, setNameError] = useState<string>();
  const [formError, setFormError] = useState<string>();
  const [pending, setPending] = useState(false);
  const zones = useMemo(() => timezoneChoices(user.timezone), [user.timezone]);

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    const trimmed = name.trim();
    if (!trimmed) return setNameError(t("nameRequired"));
    if ([...trimmed].length > 80) return setNameError(t("nameLong"));
    setNameError(undefined);
    setFormError(undefined);

    const body: components["schemas"]["UpdateMeRequest"] = {};
    if (trimmed !== user.display_name) body.display_name = trimmed;
    if (locale !== user.locale) body.locale = locale;
    if (timezone !== user.timezone) body.timezone = timezone;
    if (Object.keys(body).length === 0) return void toast({ title: t("nothingToSave") });

    setPending(true);
    try {
      await unwrap(api.PATCH("/me", { body }));
    } catch (err) {
      const code = err instanceof ApiError ? err.code : "unknown";
      if (code === "auth.invalid_name") setNameError(tErr(errorMessageKey(code)));
      else setFormError(tErr(errorMessageKey(code)));
      setPending(false);
      return;
    }
    if (body.locale) setLocaleCookie(body.locale);
    setPending(false);
    toast({ title: t("saved"), tone: "success" });
    router.refresh();
  }

  return (
    <form onSubmit={onSubmit} noValidate className="mt-3 flex flex-col gap-5">
      <Field label={t("name")} hint={t("nameHint")} error={nameError}>
        <Input name="name" value={name} autoComplete="name" onChange={(ev) => setName(ev.target.value)} />
      </Field>

      <div className="flex flex-col gap-1">
        <span className="text-sm font-medium text-ink">{t("email")}</span>
        <span className="break-words text-[15px] text-muted">{user.email}</span>
      </div>

      <Field label={t("language")}>
        <Select value={locale} onValueChange={(v) => setLocale(v === "en" ? "en" : "id")}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="id">{t("languageId")}</SelectItem>
            <SelectItem value="en">{t("languageEn")}</SelectItem>
          </SelectContent>
        </Select>
      </Field>

      <Field label={t("timezone")} hint={t("timezoneHint")}>
        <Select value={timezone} onValueChange={setTimezone}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {zones.map((z) => (
              <SelectItem key={z} value={z}>
                {z.replaceAll("_", " ")}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </Field>

      {formError ? <FormError>{formError}</FormError> : null}
      <div>
        <Button type="submit" loading={pending}>
          {pending ? t("saving") : t("save")}
        </Button>
      </div>
    </form>
  );
}
