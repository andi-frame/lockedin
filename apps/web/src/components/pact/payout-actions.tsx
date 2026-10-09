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
import { newIdempotencyKey } from "@/lib/api/idempotency";
import { unwrap } from "@/lib/api/unwrap";
import { NOTE_MAX, noteState } from "@/lib/pact/settlement";

/** One call and its feedback: a key per payload (a retry replays), the server's refusal in words, a refresh on success. */
function usePayoutCall() {
  const tErr = useTranslations("Errors");
  const router = useRouter();
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
  return { run, pending, error, setError };
}

/** The backer says the IOU was paid, with an optional note about how. */
export function MarkPaidButton({ pactId, doerName }: { pactId: string; doerName: string }) {
  const t = useTranslations("Settlement");
  const call = usePayoutCall();
  const [open, setOpen] = useState(false);
  const [note, setNote] = useState("");
  const state = noteState(note);

  async function submit() {
    if (!state.ok) return;
    const ok = await call.run(
      `paid:${state.value ?? ""}`,
      (key) => unwrap(api.POST("/pacts/{pactId}/payout/mark-paid", { params: { path: { pactId }, header: { "Idempotency-Key": key } }, body: { note: state.value } })),
      t("markDone"),
    );
    if (ok) setOpen(false);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (call.pending) return;
        setOpen(o);
        if (!o) call.setError(undefined);
      }}
    >
      <DialogTrigger asChild>
        <Button variant="decision">{t("markPaid")}</Button>
      </DialogTrigger>
      <DialogContent>
        <form
          noValidate
          onSubmit={(e) => {
            e.preventDefault();
            void submit();
          }}
          className="flex flex-col gap-4"
        >
          <DialogHeader>
            <DialogTitle>{t("markTitle")}</DialogTitle>
            <DialogDescription>{t("markBody", { name: doerName })}</DialogDescription>
          </DialogHeader>
          <Field label={t("noteLabel")} hint={t("noteHint")} error={state.ok ? undefined : t("noteTooLong", { max: NOTE_MAX })}>
            <Textarea value={note} onChange={(e) => setNote(e.target.value)} rows={3} autoComplete="off" autoFocus />
          </Field>
          <p className="-mt-2 font-mono text-[13px] tabular-nums text-muted">{t("noteCount", { n: state.length, max: NOTE_MAX })}</p>
          {call.error ? <FormError>{call.error}</FormError> : null}
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="secondary" disabled={call.pending}>
                {t("cancel")}
              </Button>
            </DialogClose>
            <Button type="submit" variant="decision" disabled={!state.ok} loading={call.pending}>
              {t("markPaid")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/** The doer confirms receipt. Alone, this completes the pact, so it asks first and says when the backer has not marked it paid. */
export function ConfirmReceiptButton({ pactId, backerMarkedPaid }: { pactId: string; backerMarkedPaid: boolean }) {
  const t = useTranslations("Settlement");
  const call = usePayoutCall();
  const [open, setOpen] = useState(false);

  async function submit() {
    const ok = await call.run(
      "confirm",
      (key) => unwrap(api.POST("/pacts/{pactId}/payout/confirm", { params: { path: { pactId }, header: { "Idempotency-Key": key } } })),
      t("confirmDone"),
    );
    if (ok) setOpen(false);
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(o) => {
        if (call.pending) return;
        setOpen(o);
        if (!o) call.setError(undefined);
      }}
    >
      <DialogTrigger asChild>
        <Button variant="decision">{t("confirm")}</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("confirmTitle")}</DialogTitle>
          <DialogDescription>{t("confirmBody")}</DialogDescription>
        </DialogHeader>
        {backerMarkedPaid ? null : <p className="text-sm font-medium text-stamp-text">{t("confirmNotPaid")}</p>}
        {call.error ? <FormError>{call.error}</FormError> : null}
        <DialogFooter>
          <DialogClose asChild>
            <Button type="button" variant="secondary" disabled={call.pending}>
              {t("cancel")}
            </Button>
          </DialogClose>
          <Button variant="decision" loading={call.pending} onClick={() => void submit()}>
            {t("confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
