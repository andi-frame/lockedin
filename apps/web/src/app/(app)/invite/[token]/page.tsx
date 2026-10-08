import type { Metadata } from "next";
import { getTranslations } from "next-intl/server";
import Link from "next/link";
import { InviteAccept } from "@/components/pact/invite-accept";
import { TermsSummary } from "@/components/pact/terms-summary";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/errors";
import { serverApi } from "@/lib/api/server";
import { unwrap } from "@/lib/api/unwrap";
import { getCurrentUser } from "@/lib/auth/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Invite");
  return { title: t("title") };
}

function Unusable({ title, body, cta }: { title: string; body: string; cta: string }) {
  return (
    <section className="max-w-xl">
      <h1 className="text-3xl font-semibold tracking-tight">{title}</h1>
      <p className="mb-6 mt-3 text-[15px] text-muted">{body}</p>
      <Button asChild>
        <Link href="/pacts">{cta}</Link>
      </Button>
    </section>
  );
}

// An invited person lands here from the email or from a link the backer shared. The proxy has
// already sent a signed-out visitor to /login?next=/invite/<token>, so there is a user by now.
export default async function InvitePage({ params }: { params: Promise<{ token: string }> }) {
  const { token } = await params;
  const [t, user, api] = await Promise.all([getTranslations("Invite"), getCurrentUser(), serverApi()]);
  if (!user) return null;

  const preview = await unwrap(api.GET("/invites/{token}", { params: { path: { token } } })).catch((err) => {
    if (err instanceof ApiError && err.code === "pact.invite_invalid") return null;
    throw err;
  });
  if (!preview || preview.status !== "proposed" || !preview.doer_slot_open) {
    return <Unusable title={t("invalidTitle")} body={t("invalidBody")} cta={t("toPacts")} />;
  }
  if (preview.inviter.user_id === user.id) {
    return <Unusable title={t("ownTitle")} body={t("ownBody")} cta={t("toPacts")} />;
  }

  // The invitee is named by their own display name: it is also the name they will type to sign.
  const names = { backer: preview.inviter.display_name, doer: user.display_name };
  return (
    <>
      <h1 className="text-3xl font-semibold tracking-tight [overflow-wrap:anywhere]">{preview.pact_title}</h1>
      <p className="mt-2 max-w-prose text-[15px] text-muted">{t("intro", { name: preview.inviter.display_name })}</p>

      <section aria-labelledby="terms" className="mt-8 max-w-2xl">
        <h2 id="terms" className="mb-3 text-lg font-semibold tracking-tight">
          {t("termsTitle")}
        </h2>
        <TermsSummary terms={preview.terms} names={names} />
      </section>

      <section aria-labelledby="sign" className="mt-8">
        <h2 id="sign" className="text-lg font-semibold tracking-tight">
          {t("signTitle")}
        </h2>
        <p className="mb-4 mt-1 max-w-prose text-[15px] text-muted">{t("signIntro")}</p>
        <InviteAccept token={token} displayName={user.display_name} />
      </section>
    </>
  );
}
