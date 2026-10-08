"use client";

import { Check, Copy } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { inviteUrl } from "@/lib/pact/agreement";

/** The invite link, shown with a copy button. The token exists in full only in this response. */
export function InviteLink({ token }: { token: string }) {
  const t = useTranslations("Invite");
  const input = useRef<HTMLInputElement>(null);
  const [copied, setCopied] = useState(false);
  // Only rendered after a click, never on the server, so reading the origin here is safe.
  const url = inviteUrl(window.location.origin, token);

  async function copy() {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2500);
    } catch {
      // No clipboard permission: select the text so the person can copy it by hand.
      input.current?.select();
    }
  }

  return (
    <Field label={t("linkLabel")} hint={t("linkHint")}>
      <div className="flex flex-col gap-2 sm:flex-row">
        <Input ref={input} readOnly value={url} onFocus={(ev) => ev.currentTarget.select()} className="font-mono text-sm" />
        <Button variant="secondary" onClick={copy} className="sm:min-w-36">
          {copied ? <Check aria-hidden weight="bold" className="size-4" /> : <Copy aria-hidden weight="bold" className="size-4" />}
          {copied ? t("copied") : t("copy")}
        </Button>
      </div>
      <p role="status" className="sr-only">
        {copied ? t("copied") : ""}
      </p>
    </Field>
  );
}

/** What the backer sees right after proposing. */
export function InviteShare({ pactId, token, emailed, email }: { pactId: string; token: string; emailed: boolean; email: string }) {
  const t = useTranslations("Invite");
  return (
    <section aria-labelledby="proposed" className="flex max-w-2xl flex-col gap-6">
      <div>
        <h2 id="proposed" tabIndex={-1} className="text-2xl font-semibold tracking-tight">
          {t("proposedTitle")}
        </h2>
        <p className="mt-2 text-[15px] text-muted">{emailed ? t("proposedEmailed", { email }) : t("proposedBody")}</p>
      </div>
      <InviteLink token={token} />
      <p className="text-sm text-muted">{t("onceNote")}</p>
      <div>
        <Button asChild>
          <Link href={`/pacts/${pactId}`}>{t("openPact")}</Link>
        </Button>
      </div>
    </section>
  );
}
