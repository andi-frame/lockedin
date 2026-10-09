"use client";

import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import { unwrap } from "@/lib/api/unwrap";
import { EMAIL_KINDS, emailKindMessage, type EmailKind } from "@/lib/settings/email-kinds";

/**
 * Which emails to get. The list on the server holds what is switched OFF, so the boxes are
 * ticked for "send me this". What cannot be switched off is written below the boxes, with why.
 */
export function EmailForm({ off }: { off: EmailKind[] }) {
  const t = useTranslations("Settings");
  const tErr = useTranslations("Errors");
  const router = useRouter();
  const [wanted, setWanted] = useState<Set<EmailKind>>(() => new Set(EMAIL_KINDS.filter((k) => !off.includes(k))));
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);

  function toggle(kind: EmailKind, on: boolean) {
    setWanted((prev) => {
      const next = new Set(prev);
      if (on) next.add(kind);
      else next.delete(kind);
      return next;
    });
  }

  async function onSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (pending) return;
    setError(undefined);
    setPending(true);
    try {
      await unwrap(api.PATCH("/me", { body: { email_kinds_off: EMAIL_KINDS.filter((k) => !wanted.has(k)) } }));
    } catch (err) {
      setError(tErr(errorMessageKey(err instanceof ApiError ? err.code : "unknown")));
      setPending(false);
      return;
    }
    setPending(false);
    toast({ title: t("saved"), tone: "success" });
    router.refresh();
  }

  return (
    <form onSubmit={onSubmit} className="mt-3 flex flex-col gap-4">
      <fieldset className="flex flex-col">
        <legend className="mb-1 text-sm font-medium text-ink">{t("emailLegend")}</legend>
        {EMAIL_KINDS.map((kind) => (
          <label key={kind} className="flex min-h-11 cursor-pointer items-start gap-3 py-2 text-[15px]">
            <input type="checkbox" name={kind} checked={wanted.has(kind)} onChange={(ev) => toggle(kind, ev.target.checked)} className="mt-0.5 size-5 shrink-0 accent-primary" />
            <span>{t(emailKindMessage(kind))}</span>
          </label>
        ))}
      </fieldset>
      <p className="max-w-prose text-sm text-muted">{t("emailMustStay")}</p>
      {error ? <FormError>{error}</FormError> : null}
      <div>
        <Button type="submit" loading={pending}>
          {pending ? t("saving") : t("save")}
        </Button>
      </div>
    </form>
  );
}
