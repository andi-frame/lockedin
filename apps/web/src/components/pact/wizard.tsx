"use client";

import { ArrowLeft, ArrowRight, PaperPlaneTilt } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState, type FormEvent } from "react";
import { FormError, focusFirstInvalid } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import type { components } from "@/lib/api/schema";
import { unwrap } from "@/lib/api/unwrap";
import { addDays, todayIn } from "@/lib/pact/dates";
import { firstInvalidStep, steps, toPactDraft, validateStep, type Draft, type DraftErrors, type MemberDraft } from "@/lib/pact/draft";
import { InviteShare } from "./invite-share";
import { TermsSummary } from "./terms-summary";
import { BasicsStep, CommitmentStep, RulesStep, type StepProps } from "./wizard-fields";
import { useUnsavedChangesWarning } from "@/lib/unsaved";

type Pact = components["schemas"]["Pact"];

const pages = [...steps, "review"] as const;
type Page = (typeof pages)[number];

type Props = {
  me: { id: string; displayName: string };
  initial: Draft;
  /** Present when editing a draft or proposed pact. */
  pact?: Pact;
};

const emailShape = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function Wizard({ me, initial, pact }: Props) {
  const t = useTranslations();
  const router = useRouter();
  const formRef = useRef<HTMLFormElement>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const [now] = useState(() => new Date());
  const [draft, setDraft] = useState<Draft>(initial);
  const [index, setIndex] = useState(0);
  const [errors, setErrors] = useState<DraftErrors>({});
  const [formError, setFormError] = useState<string>();
  const [email, setEmail] = useState("");
  const [emailError, setEmailError] = useState<string>();
  const [pending, setPending] = useState(false);
  const [invite, setInvite] = useState<{ pactId: string; token: string; emailed: boolean }>();
  // A half-filled pact is lost if the tab is closed or reloaded: the browser asks first. Done once the invite is out.
  const [initialDraft] = useState(() => JSON.stringify(initial));
  useUnsavedChangesWarning(!invite && JSON.stringify(draft) !== initialDraft);

  // A retry of the same action reuses its key so the server replays instead of acting twice; an
  // edit between attempts is a different action and gets a new key.
  const created = useRef<{ pactId: string; payload: string }>(undefined);
  const keys = useRef<Record<string, { payload: string; key: string }>>({});
  const keyFor = (action: string, payload: string) => {
    const known = keys.current[action];
    if (known?.payload === payload) return known.key;
    const key = crypto.randomUUID();
    keys.current[action] = { payload, key };
    return key;
  };

  const page: Page = pages[index] ?? "basics";
  const editing = Boolean(pact);
  // Either member may edit before both have signed (SPEC §3), so when editing, the ids and names
  // come from the pact and not from who is looking.
  const backer = pact?.members.find((m) => m.role === "backer");
  const doer = pact?.members.find((m) => m.role === "doer");
  const names = { backer: backer?.display_name ?? me.displayName, doer: doer?.display_name ?? t("Role.doer") };
  const ids = { backer: backer?.user_id ?? me.id, doer: doer?.user_id };

  // Move focus to the new step's heading so a keyboard or screen-reader user starts at its top.
  useEffect(() => {
    if (index > 0) headingRef.current?.focus();
  }, [index]);

  // Editing a field retires its own error, so a fixed field stops shouting.
  const clearErrors = (keys: string[]) =>
    setErrors((e) => (keys.some((k) => k in e) ? Object.fromEntries(Object.entries(e).filter(([k]) => !keys.includes(k))) : e));
  const set = (patch: Partial<Draft>) => {
    setDraft((d) => ({ ...d, ...patch }));
    setFormError(undefined);
    clearErrors(Object.keys(patch));
  };
  const setMember = (who: "backer" | "doer", patch: Partial<MemberDraft>) => {
    setDraft((d) => ({ ...d, [who]: { ...d[who], ...patch } }));
    setFormError(undefined);
    clearErrors(Object.keys(patch).map((k) => `${who}.${k}`));
  };

  const stepProps: StepProps = {
    draft,
    errors,
    minStart: addDays(todayIn(draft.timezone, now), 1),
    names,
    set,
    setMember,
  };

  function go(to: number) {
    setErrors({});
    setIndex(to);
  }

  function failOn(step: (typeof steps)[number], found: DraftErrors) {
    setErrors(found);
    setIndex(steps.indexOf(step));
    // The step has to render before its fields exist to focus.
    requestAnimationFrame(() => {
      if (formRef.current) focusFirstInvalid(formRef.current, Object.keys(found));
    });
  }

  function onSubmit(event: FormEvent) {
    event.preventDefault();
    if (pending) return;
    if (page !== "review") {
      const found = validateStep(page, draft, now);
      if (Object.keys(found).length) {
        setErrors(found);
        if (formRef.current) focusFirstInvalid(formRef.current, Object.keys(found));
        return;
      }
      go(index + 1);
      return;
    }
    void submit();
  }

  async function submit() {
    const bad = firstInvalidStep(draft, now);
    if (bad) {
      setFormError(t("Wizard.fixFirst"));
      failOn(bad.step, bad.errors);
      return;
    }
    const to = email.trim();
    if (!editing && to && !emailShape.test(to)) {
      setEmailError(t("Auth.emailInvalid"));
      return;
    }
    setEmailError(undefined);
    setFormError(undefined);
    setPending(true);
    try {
      const body = toPactDraft(draft, ids);
      const payload = JSON.stringify(body);
      if (pact) {
        await unwrap(
          api.PATCH("/pacts/{pactId}", { params: { path: { pactId: pact.id }, header: { "Idempotency-Key": keyFor("update", payload) } }, body }),
        );
        router.replace(`/pacts/${pact.id}?edited=1`);
        router.refresh();
        return;
      }
      let pactId = created.current?.pactId;
      if (!pactId) {
        const draftPact = await unwrap(api.POST("/pacts", { params: { header: { "Idempotency-Key": keyFor("create", payload) } }, body }));
        pactId = draftPact.id;
        created.current = { pactId, payload };
      } else if (created.current?.payload !== payload) {
        // Propose failed once and the person changed something since: save that before inviting.
        await unwrap(api.PATCH("/pacts/{pactId}", { params: { path: { pactId }, header: { "Idempotency-Key": keyFor("update", payload) } }, body }));
        created.current = { pactId, payload };
      }
      const proposal = await unwrap(
        api.POST("/pacts/{pactId}/propose", {
          params: { path: { pactId }, header: { "Idempotency-Key": keyFor("propose", `${pactId}|${to}`) } },
          body: to ? { email: to } : undefined,
        }),
      );
      setInvite({ pactId, token: proposal.invite_token, emailed: Boolean(to) });
    } catch (err) {
      setFormError(t(`Errors.${errorMessageKey(err instanceof ApiError ? err.code : "unknown")}`));
      setPending(false);
    }
  }

  if (invite) return <InviteShare pactId={invite.pactId} token={invite.token} emailed={invite.emailed} email={email.trim()} />;

  return (
    <form ref={formRef} onSubmit={onSubmit} noValidate className="flex max-w-2xl flex-col gap-6">
      <nav aria-label={t("Wizard.stepsLabel")}>
        <ol className="grid grid-cols-4 gap-1.5">
          {pages.map((p, i) => (
            <li key={p} aria-current={i === index ? "step" : undefined}>
              <span className={`block h-1 rounded-full ${i <= index ? "bg-primary" : "bg-rule-strong"}`} />
              <span className="sr-only">{t(`Wizard.step_${p}`)}</span>
            </li>
          ))}
        </ol>
        <p className="mt-2 text-sm text-muted" aria-hidden>
          {t("Wizard.stepCount", { n: index + 1, total: pages.length })}
        </p>
      </nav>

      <h2 ref={headingRef} tabIndex={-1} className="text-2xl font-semibold tracking-tight outline-none">
        {t(`Wizard.title_${page}`)}
      </h2>

      {page === "basics" ? <BasicsStep {...stepProps} /> : null}
      {page === "commitment" ? <CommitmentStep {...stepProps} /> : null}
      {page === "rules" ? <RulesStep {...stepProps} /> : null}
      {page === "review" ? (
        <div className="flex flex-col gap-6">
          <div>
            <p className="mb-3 max-w-prose text-[15px] text-muted">{t("Wizard.reviewIntro")}</p>
            <h3 className="mb-2 text-lg font-semibold tracking-tight">{draft.title.trim()}</h3>
            <TermsSummary terms={toPactDraft(draft, ids).terms} names={names} />
          </div>
          {editing ? (
            pact?.status === "proposed" ? (
              <p className="rounded-control border border-rule-strong bg-sunken px-3 py-2.5 text-sm">{t("Wizard.editWarning")}</p>
            ) : null
          ) : (
            <Field label={t("Wizard.emailLabel")} hint={t("Wizard.emailHint")} error={emailError}>
              <Input name="email" type="email" inputMode="email" autoComplete="off" autoCapitalize="none" spellCheck={false} value={email} onChange={(ev) => setEmail(ev.target.value)} />
            </Field>
          )}
        </div>
      ) : null}

      {formError ? <FormError>{formError}</FormError> : null}

      <div className="flex flex-col-reverse gap-3 sm:flex-row sm:justify-between">
        {index > 0 ? (
          <Button variant="secondary" onClick={() => go(index - 1)} disabled={pending}>
            <ArrowLeft aria-hidden weight="bold" className="size-4" />
            {t("Wizard.back")}
          </Button>
        ) : (
          <span />
        )}
        <Button type="submit" loading={pending} className="sm:min-w-44">
          {page === "review" ? (
            <>
              <PaperPlaneTilt aria-hidden weight="bold" className="size-4" />
              {pending ? t("Wizard.sending") : editing ? t("Wizard.save") : t("Wizard.propose")}
            </>
          ) : (
            <>
              {t("Wizard.next")}
              <ArrowRight aria-hidden weight="bold" className="size-4" />
            </>
          )}
        </Button>
      </div>
    </form>
  );
}
