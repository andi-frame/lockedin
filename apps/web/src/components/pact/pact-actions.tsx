"use client";

import { LinkSimple } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import { unwrap } from "@/lib/api/unwrap";
import { InviteLink } from "./invite-share";
import { SignForm } from "./sign-form";

const messageFor = (t: (k: string) => string, err: unknown) => t(`Errors.${errorMessageKey(err instanceof ApiError ? err.code : "unknown")}`);

/** Sign the current terms. Signing is a human decision, so the form wears the stamp colour. */
export function SignPanel({ pactId, termsHash, displayName }: { pactId: string; termsHash: string; displayName: string }) {
  const t = useTranslations();
  const router = useRouter();
  return (
    <SignForm
      displayName={displayName}
      label={t("Sign.submit")}
      onSign={async (typed) => {
        try {
          await unwrap(api.POST("/pacts/{pactId}/accept", { params: { path: { pactId } }, body: { terms_hash: termsHash, signature_name: typed } }));
        } catch (err) {
          // Someone edited the terms while this page was open: reload so the new text is what gets signed.
          if (err instanceof ApiError && err.code === "pact.terms_mismatch") router.refresh();
          throw new Error(messageFor(t, err));
        }
        toast({ title: t("Sign.signed"), tone: "success" });
        router.refresh();
      }}
    />
  );
}

/** Backer tools while the pact is still being agreed: mint an invite link. */
export function BackerTools({ pactId, status }: { pactId: string; status: "draft" | "proposed" }) {
  const t = useTranslations();
  const [token, setToken] = useState<string>();
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();
  const router = useRouter();

  async function invite() {
    if (pending) return;
    setPending(true);
    setError(undefined);
    try {
      const proposal = await unwrap(api.POST("/pacts/{pactId}/propose", { params: { path: { pactId } } }));
      setToken(proposal.invite_token);
      if (status === "draft") router.refresh();
    } catch (err) {
      setError(messageFor(t, err));
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <div>
        <Button variant="secondary" onClick={invite} loading={pending}>
          <LinkSimple aria-hidden weight="bold" className="size-4" />
          {status === "draft" ? t("PactPage.propose") : t("PactPage.newInvite")}
        </Button>
      </div>
      {error ? <FormError>{error}</FormError> : null}
      {token ? (
        <div className="max-w-xl">
          <InviteLink token={token} />
          <p className="mt-2 text-sm text-muted">{t("Invite.newLinkNote")}</p>
        </div>
      ) : null}
    </div>
  );
}
