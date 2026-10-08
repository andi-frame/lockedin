"use client";

import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { toast } from "@/components/ui/toast";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import { unwrap } from "@/lib/api/unwrap";
import { SignForm } from "./sign-form";

/**
 * Joining and signing are two API calls (`joinInvite` re-keys the doer slot, which changes the terms
 * hash; `acceptPact` then signs the new one). The person sees one action: sign.
 */
export function InviteAccept({ token, displayName }: { token: string; displayName: string }) {
  const t = useTranslations();
  const router = useRouter();
  const messageFor = (err: unknown) => t(`Errors.${errorMessageKey(err instanceof ApiError ? err.code : "unknown")}`);

  return (
    <SignForm
      displayName={displayName}
      label={t("Invite.joinAndSign")}
      onSign={async (typed) => {
        let pactId: string;
        let termsHash: string;
        try {
          const joined = await unwrap(api.POST("/invites/{token}/join", { params: { path: { token } } }));
          pactId = joined.id;
          termsHash = joined.terms_hash;
        } catch (err) {
          throw new Error(messageFor(err));
        }
        try {
          await unwrap(api.POST("/pacts/{pactId}/accept", { params: { path: { pactId } }, body: { terms_hash: termsHash, signature_name: typed } }));
          toast({ title: t("Sign.signed"), tone: "success" });
        } catch (err) {
          // Already a member, so the sign form on the pact page is the way forward.
          toast({ title: t("Invite.joinedNotSigned"), description: messageFor(err), tone: "error" });
        }
        router.replace(`/pacts/${pactId}`);
        router.refresh();
      }}
    />
  );
}
