import { CheckCircle, Circle, PencilSimple } from "@phosphor-icons/react/dist/ssr";
import type { Metadata } from "next";
import { getFormatter, getTranslations } from "next-intl/server";
import Link from "next/link";
import { notFound } from "next/navigation";
import { MemberLine } from "@/components/member-line";
import { BackerTools, SignPanel } from "@/components/pact/pact-actions";
import { PactStatusBadge } from "@/components/pact/status-badge";
import { TermsSummary } from "@/components/pact/terms-summary";
import { Button } from "@/components/ui/button";
import { ApiError } from "@/lib/api/errors";
import { serverApi } from "@/lib/api/server";
import { unwrap } from "@/lib/api/unwrap";
import { getCurrentUser } from "@/lib/auth/session";
import { memberSlot } from "@/lib/member";
import { agreementState } from "@/lib/pact/agreement";

export async function generateMetadata({ params }: { params: Promise<{ id: string }> }): Promise<Metadata> {
  const { id } = await params;
  const api = await serverApi();
  const pact = await unwrap(api.GET("/pacts/{pactId}", { params: { path: { pactId: id } } })).catch(() => null);
  return { title: pact?.title ?? (await getTranslations("Pacts"))("title") };
}

// The agreement view: who has signed, the terms in plain language, and what each person can do next.
// The passbook and calendar for a running pact arrive with task 6.4.
export default async function PactPage({
  params,
  searchParams,
}: {
  params: Promise<{ id: string }>;
  searchParams: Promise<{ edited?: string }>;
}) {
  const [{ id }, { edited }] = await Promise.all([params, searchParams]);
  const [t, format, user, api] = await Promise.all([getTranslations("PactPage"), getFormatter(), getCurrentUser(), serverApi()]);
  if (!user) return null;
  const pact = await unwrap(api.GET("/pacts/{pactId}", { params: { path: { pactId: id } } })).catch((err) => {
    // A non-member gets 404 (AGENTS invariant 8); a malformed id looks the same to the visitor.
    if (err instanceof ApiError && (err.status === 404 || err.status === 400)) notFound();
    throw err;
  });

  const state = agreementState(pact, user.id);
  const backer = pact.members.find((m) => m.role === "backer");
  const doer = pact.members.find((m) => m.role === "doer");
  const names = { backer: backer?.display_name ?? "", doer: doer?.display_name ?? t("doerPlaceholder") };
  const ids = pact.members.map((m) => m.user_id);
  const date = (iso: string) => format.dateTime(new Date(`${iso}T00:00:00Z`), { dateStyle: "long", timeZone: "UTC" });
  const editable = pact.status === "draft" || pact.status === "proposed";

  return (
    <>
      <div className="flex max-w-2xl flex-wrap items-start justify-between gap-x-4 gap-y-2">
        <h1 className="min-w-0 text-3xl font-semibold tracking-tight [overflow-wrap:anywhere]">{pact.title}</h1>
        <PactStatusBadge status={pact.status} />
      </div>
      <p className="mt-2 font-mono text-sm tabular-nums text-muted">
        {date(pact.starts_on)} – {date(pact.ends_on)}
      </p>
      {pact.description ? <p className="mt-4 max-w-prose text-[15px]">{pact.description}</p> : null}

      {edited ? (
        <p role="status" className="mt-6 max-w-2xl rounded-control border border-stamp-text bg-stamp-tint px-3 py-2.5 text-sm font-medium text-stamp-text">
          {t("edited")}
        </p>
      ) : null}

      <section aria-labelledby="signatures" className="mt-8 max-w-2xl">
        <h2 id="signatures" className="text-lg font-semibold tracking-tight">
          {t("signatures")}
        </h2>
        {state.termsChanged ? <p className="mt-2 text-sm text-muted">{t("termsChanged")}</p> : null}
        <ul className="mt-3 divide-y divide-rule border-y border-rule">
          {pact.members.map((m) => (
            <li key={m.user_id} className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-3">
              <MemberLine name={m.display_name} slot={memberSlot(m.user_id, ids)} role={m.role} />
              <span className="inline-flex items-center gap-2 text-sm">
                {m.accepted ? (
                  <>
                    <CheckCircle aria-hidden weight="fill" className="size-5 text-stamp-text" />
                    <span className="font-medium">
                      {t("signed", { name: m.signature_name ?? m.display_name })}
                      {m.accepted_at ? (
                        <span className="font-mono tabular-nums text-muted">
                          {" · "}
                          {format.dateTime(new Date(m.accepted_at), { dateStyle: "medium", timeStyle: "short", timeZone: pact.timezone })}
                        </span>
                      ) : null}
                    </span>
                  </>
                ) : (
                  <>
                    <Circle aria-hidden className="size-5 text-muted" />
                    <span className="text-muted">{t("notSigned")}</span>
                  </>
                )}
              </span>
            </li>
          ))}
          {!doer ? (
            <li className="flex items-center gap-2 py-3 text-sm text-muted">
              <Circle aria-hidden className="size-5" />
              {t("waitingForDoer")}
            </li>
          ) : null}
        </ul>
      </section>

      {pact.my_role === "backer" && editable && state.phase !== "signing" ? (
        <section aria-labelledby="invite" className="mt-8">
          <h2 id="invite" className="mb-3 text-lg font-semibold tracking-tight">
            {t("inviteTitle")}
          </h2>
          <BackerTools pactId={pact.id} status={pact.status === "draft" ? "draft" : "proposed"} />
        </section>
      ) : null}

      <section aria-labelledby="terms" className="mt-8 max-w-2xl">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <h2 id="terms" className="text-lg font-semibold tracking-tight">
            {t("terms")}
          </h2>
          {editable ? (
            <Button variant="ghost" size="sm" asChild>
              <Link href={`/pacts/${pact.id}/edit`}>
                <PencilSimple aria-hidden weight="bold" className="size-4" />
                {t("edit")}
              </Link>
            </Button>
          ) : null}
        </div>
        <TermsSummary terms={pact.terms} names={names} />
      </section>

      {state.canSign ? (
        <section aria-labelledby="sign" className="mt-8">
          <h2 id="sign" className="text-lg font-semibold tracking-tight">
            {t("signTitle")}
          </h2>
          <p className="mb-4 mt-1 max-w-prose text-[15px] text-muted">{t("signIntro")}</p>
          <SignPanel pactId={pact.id} termsHash={pact.terms_hash} displayName={user.display_name} />
        </section>
      ) : null}

      {state.phase === "signing" && state.iSigned ? <p className="mt-8 max-w-prose text-[15px] text-muted">{t("waitingForOther")}</p> : null}

      {pact.my_role === "backer" && editable && state.phase === "signing" ? (
        <section aria-labelledby="invite" className="mt-8">
          <h2 id="invite" className="mb-3 text-lg font-semibold tracking-tight">
            {t("inviteTitle")}
          </h2>
          <BackerTools pactId={pact.id} status={pact.status === "draft" ? "draft" : "proposed"} />
        </section>
      ) : null}

      {pact.status === "scheduled" ? <p className="mt-8 max-w-prose text-[15px]">{t("scheduled", { date: date(pact.starts_on) })}</p> : null}
      {pact.status === "active" || pact.status === "settling" || pact.status === "completed" ? (
        <p className="mt-8 max-w-prose text-[15px] text-muted">{t("laterScreens")}</p>
      ) : null}
    </>
  );
}
