"use client";

import { useTranslations } from "next-intl";
import { useRouter } from "next/navigation";
import { useRef, useState } from "react";
import { FormError } from "@/components/auth/form-error";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Field } from "@/components/ui/field";
import { Textarea } from "@/components/ui/input";
import { toast } from "@/components/ui/toast";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import type { components } from "@/lib/api/schema";
import { newIdempotencyKey } from "@/lib/api/idempotency";
import { unwrap } from "@/lib/api/unwrap";
import { REASON_MAX, REASON_MIN, reasonState } from "@/lib/checkin/reason";

type Action = components["schemas"]["CheckInAction"];
type Outcome = "uphold" | "dismiss";

/** The actions that are decisions about a proof. Sending and resting belong to the doer's own page. */
const decisions = ["approve", "reject", "override", "dispute", "resolve_dispute"] as const satisfies readonly Action[];
type Decision = (typeof decisions)[number];

/**
 * The buttons for what the server says this person may do right now (`my_actions`); nothing is
 * decided here. These are human decisions, so they wear the stamp. Everything that needs a reason
 * asks for it in a dialog, and a refusal from the server is shown there, in words.
 */
export function CheckInActions({ checkInId, actions, overridesRemaining }: { checkInId: string; actions: Action[]; overridesRemaining: number | null }) {
  const offered = decisions.filter((d) => actions.includes(d));
  if (offered.length === 0) return null;
  return (
    <div className="flex flex-wrap gap-3" data-testid="decision-actions">
      {offered.map((d) =>
        d === "approve" ? (
          <ApproveButton key={d} checkInId={checkInId} />
        ) : (
          <ReasonDialog key={d} action={d} checkInId={checkInId} overridesRemaining={overridesRemaining} />
        ),
      )}
    </div>
  );
}

function useDecision(checkInId: string) {
  const tErr = useTranslations("Errors");
  const router = useRouter();
  // One key per payload: a retry of the same decision replays it, a changed payload is a new one.
  const attempt = useRef<{ payload: string; key: string }>({ payload: "", key: newIdempotencyKey() });
  const [pending, setPending] = useState(false);
  const [error, setError] = useState<string>();

  async function run(payload: string, call: (key: string) => Promise<unknown>, done: string): Promise<boolean> {
    if (pending) return false;
    if (attempt.current.payload !== payload) attempt.current = { payload, key: newIdempotencyKey() };
    setPending(true);
    setError(undefined);
    try {
      await call(attempt.current.key);
    } catch (err) {
      setError(tErr(errorMessageKey(err instanceof ApiError ? err.code : "unknown")));
      setPending(false);
      return false;
    }
    setPending(false);
    toast({ title: done, tone: "success" });
    router.refresh();
    return true;
  }
  return { run, pending, error, setError, checkInId };
}

function ApproveButton({ checkInId }: { checkInId: string }) {
  const t = useTranslations("Detail");
  const d = useDecision(checkInId);
  return (
    <div className="flex flex-col gap-2">
      <Button
        variant="decision"
        loading={d.pending}
        onClick={() =>
          d.run(
            "approve",
            (key) => unwrap(api.POST("/check-ins/{checkInId}/approve", { params: { path: { checkInId }, header: { "Idempotency-Key": key } } })),
            t("approveDone"),
          )
        }
      >
        {t("approve")}
      </Button>
      {d.error ? <FormError>{d.error}</FormError> : null}
    </div>
  );
}

function ReasonDialog({ action, checkInId, overridesRemaining }: { action: Exclude<Decision, "approve">; checkInId: string; overridesRemaining: number | null }) {
  const t = useTranslations("Detail");
  const d = useDecision(checkInId);
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState("");
  const [outcome, setOutcome] = useState<Outcome>("uphold");
  const state = reasonState(reason);
  const copy = {
    reject: { title: t("rejectTitle"), body: t("rejectBody"), button: t("reject"), done: t("rejectDone") },
    override: { title: t("overrideTitle"), body: t("overrideBody"), button: t("override"), done: t("overrideDone") },
    dispute: { title: t("disputeTitle"), body: t("disputeBody"), button: t("dispute"), done: t("disputeDone") },
    resolve_dispute: { title: t("resolveTitle"), body: t("resolveBody"), button: t("resolve"), done: t("resolveDone") },
  }[action];

  async function submit() {
    if (!state.ok) return;
    const header = (key: string) => ({ "Idempotency-Key": key });
    const path = { checkInId };
    const reasonText = state.trimmed;
    const ok = await d.run(
      `${action}:${outcome}:${reasonText}`,
      (key) => {
        switch (action) {
          case "reject":
            return unwrap(api.POST("/check-ins/{checkInId}/reject", { params: { path, header: header(key) }, body: { reason: reasonText } }));
          case "override":
            return unwrap(api.POST("/check-ins/{checkInId}/override", { params: { path, header: header(key) }, body: { reason: reasonText } }));
          case "dispute":
            return unwrap(api.POST("/check-ins/{checkInId}/dispute", { params: { path, header: header(key) }, body: { reason: reasonText } }));
          case "resolve_dispute":
            return unwrap(api.POST("/check-ins/{checkInId}/dispute/resolve", { params: { path, header: header(key) }, body: { outcome, reason: reasonText } }));
        }
      },
      copy.done,
    );
    if (ok) {
      setOpen(false);
      setReason("");
    }
  }

  // The backer's powers (override, resolving a dispute) are the stamp's loud form; rejecting and
  // disputing are decisions too, drawn quieter.
  const loud = action === "override" || action === "resolve_dispute";
  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (d.pending) return;
        setOpen(o);
        if (!o) d.setError(undefined);
      }}
    >
      <DialogTrigger asChild>
        <Button variant={loud ? "decision" : "decision-quiet"}>{copy.button}</Button>
      </DialogTrigger>
      <DialogContent>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
          noValidate
          className="flex flex-col gap-4"
        >
          <DialogHeader>
            <DialogTitle>{copy.title}</DialogTitle>
            <DialogDescription>{copy.body}</DialogDescription>
          </DialogHeader>
          {action === "override" && overridesRemaining !== null ? <p className="text-sm font-medium text-stamp-text">{t("overridesLeft", { n: overridesRemaining })}</p> : null}

          {action === "resolve_dispute" ? (
            <fieldset className="flex flex-col gap-2">
              <legend className="mb-1 text-sm font-medium">{t("outcome")}</legend>
              {(["uphold", "dismiss"] as const).map((o) => (
                <label key={o} className="flex min-h-11 items-center gap-3 rounded-control border border-rule-strong bg-surface px-3 text-[15px]">
                  <input type="radio" name="outcome" value={o} checked={outcome === o} onChange={() => setOutcome(o)} className="size-4" />
                  {t(o)}
                </label>
              ))}
            </fieldset>
          ) : null}

          <Field
            label={t("reasonLabel")}
            hint={t("reasonHint", { min: REASON_MIN })}
            error={reason !== "" && !state.ok ? (state.tooLong ? t("reasonTooLong", { max: REASON_MAX }) : t("reasonMissing", { n: state.missing })) : undefined}
          >
            <Textarea value={reason} onChange={(e) => setReason(e.target.value)} autoFocus rows={4} />
          </Field>
          <p className="-mt-2 font-mono text-[13px] tabular-nums text-muted">{t("reasonCount", { n: state.length, max: REASON_MAX })}</p>

          {d.error ? <FormError>{d.error}</FormError> : null}
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="secondary" disabled={d.pending}>
                {t("cancel")}
              </Button>
            </DialogClose>
            <Button type="submit" variant={loud ? "decision" : "decision-quiet"} disabled={!state.ok} loading={d.pending}>
              {copy.button}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
