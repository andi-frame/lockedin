"use client";

import { CheckCircle, Circle, PaperPlaneTilt } from "@phosphor-icons/react";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useRef, useState, type ClipboardEvent } from "react";
import { FormError } from "@/components/auth/form-error";
import { Countdown } from "@/components/countdown";
import { Button } from "@/components/ui/button";
import { toast } from "@/components/ui/toast";
import { api } from "@/lib/api/browser";
import { ApiError, errorMessageKey } from "@/lib/api/errors";
import { unwrap } from "@/lib/api/unwrap";
import { clockIn } from "@/lib/pact/dates";
import { analyzeDoc, type ProofDoc } from "@/lib/proof/doc";
import { checkEvidence, type EvidenceRules } from "@/lib/proof/rules";
import { readyIds, trayCounts, type TrayItem } from "@/lib/proof/tray";
import { AttachmentTray } from "./attachment-tray";
import { ProofEditor } from "./proof-editor";
import { useAttachments, type Refusal } from "./use-attachments";
import { useUnsavedChangesWarning } from "@/lib/unsaved";

const empty: ProofDoc = { type: "doc", content: [{ type: "paragraph" }] };

/**
 * Write a day's proof and send it. The button stays off until the evidence rules pass and every
 * attachment is ready, and the line under it says what is still missing. The server re-checks all
 * of it, so this only keeps a person from pressing a button that is bound to fail.
 */
export function ProofForm({
  pactId,
  checkInId,
  timezone,
  commitment,
  rules,
  cutoffAt,
  submitDeadline,
  serverNow,
  initialDoc,
  initialAttachments,
  editing,
}: {
  pactId: string;
  checkInId: string;
  timezone: string;
  commitment: string;
  rules: EvidenceRules;
  cutoffAt: string;
  submitDeadline: string;
  serverNow: string;
  initialDoc: ProofDoc | null;
  initialAttachments: TrayItem[];
  editing: boolean;
}) {
  const t = useTranslations("Proof");
  const tErr = useTranslations();
  const router = useRouter();
  const tray = useAttachments(pactId, initialAttachments);
  const [doc, setDoc] = useState<ProofDoc>(initialDoc ?? empty);
  const [characters, setCharacters] = useState(0);
  const [refusals, setRefusals] = useState<Refusal[]>([]);
  const [error, setError] = useState<string>();
  const [pending, setPending] = useState(false);
  const attempt = useRef<{ payload: string; key: string }>(undefined);

  const analysis = useMemo(() => analyzeDoc(doc), [doc]);
  // Words typed or files added are lost if the tab is closed or reloaded before sending: the browser asks first.
  const [savedDoc] = useState(() => JSON.stringify(initialDoc ?? empty));
  useUnsavedChangesWarning(JSON.stringify(doc) !== savedDoc || (!editing && tray.items.length > 0));
  const words = analysis.ok ? analysis.words : 0;
  const counts = trayCounts(tray.items);
  const evidence = checkEvidence(rules, { words, ready: counts.ready, pending: counts.pending });
  const canSend = evidence.ok && analysis.ok && !pending;

  const reasons: string[] = [];
  if (!analysis.ok) reasons.push(t("whyInvalid"));
  if (evidence.needWords > 0) reasons.push(t("whyWords", { n: evidence.needWords }));
  if (evidence.needFiles > 0) reasons.push(t("whyFiles", { n: evidence.needFiles }));
  if (evidence.waiting) reasons.push(t("whyWaiting"));

  const addFiles = (files: File[]) => setRefusals(tray.add(files));

  function onPaste(e: ClipboardEvent) {
    // The editor takes its own paste; this is for files pasted while the focus is elsewhere in the form.
    if (e.defaultPrevented) return;
    const files = Array.from(e.clipboardData.files);
    if (files.length) {
      e.preventDefault();
      addFiles(files);
    }
  }

  async function send() {
    if (!canSend) return;
    setError(undefined);
    setPending(true);
    const body = { body_doc: doc, attachment_ids: readyIds(tray.items) };
    const payload = JSON.stringify(body);
    if (attempt.current?.payload !== payload) attempt.current = { payload, key: crypto.randomUUID() };
    try {
      await unwrap(
        api.PUT("/check-ins/{checkInId}/proof", { params: { path: { checkInId }, header: { "Idempotency-Key": attempt.current.key } }, body }),
      );
    } catch (err) {
      setError(tErr(`Errors.${errorMessageKey(err instanceof ApiError ? err.code : "unknown")}`));
      setPending(false);
      return;
    }
    toast({ title: editing ? t("updated") : t("sent"), tone: "success" });
    router.push("/today");
    router.refresh();
  }

  return (
    <div onPaste={onPaste} className="flex max-w-2xl flex-col gap-8">
      <section aria-labelledby="proof-task" className="flex flex-col gap-3">
        <h2 id="proof-task" className="max-w-prose text-xl font-semibold leading-snug tracking-tight [overflow-wrap:anywhere]">
          {commitment}
        </h2>
        <div className="flex flex-col gap-1">
          <Countdown until={submitDeadline} serverNow={serverNow} />
          <p className="font-mono text-sm tabular-nums text-muted">
            {t("cutoffNote", { cutoff: clockIn(cutoffAt, timezone), until: clockIn(submitDeadline, timezone) })}
          </p>
        </div>
      </section>

      <section aria-labelledby="proof-body" className="flex flex-col gap-2">
        <h3 id="proof-body" className="text-lg font-semibold tracking-tight">
          {t("bodyLabel")}
        </h3>
        <p id="proof-hint" className="text-sm text-muted">
          {t("bodyHint")}
        </p>
        <ProofEditor
          labelledBy="proof-body"
          describedBy="proof-hint proof-count"
          content={initialDoc}
          onChange={(d, c) => (setDoc(d), setCharacters(c))}
          onFiles={addFiles}
        />
        <p id="proof-count" className="font-mono text-sm tabular-nums text-muted">
          {rules.minWords > 0 ? t("wordsOf", { n: words, min: rules.minWords }) : t("words", { n: words })}
          {" · "}
          {t("characters", { n: characters })}
        </p>
      </section>

      <section aria-labelledby="proof-files" className="flex flex-col gap-2">
        <h3 id="proof-files" className="text-lg font-semibold tracking-tight">
          {t("filesLabel")}
        </h3>
        <AttachmentTray items={tray.items} refusals={refusals} onAdd={addFiles} onRetry={tray.retry} onRemove={tray.remove} labelId="proof-files-hint" />
        <p className="font-mono text-sm tabular-nums text-muted">
          {rules.minAttachments > 0 ? t("filesOf", { n: counts.ready, min: rules.minAttachments }) : t("files", { n: counts.ready })}
        </p>
      </section>

      <section aria-labelledby="proof-rules" className="flex flex-col gap-3">
        <h3 id="proof-rules" className="sr-only">
          {t("rulesLabel")}
        </h3>
        <ul className="flex flex-col gap-1.5 text-sm">
          <Rule ok={evidence.needWords === 0} text={rules.minWords > 0 ? t("ruleWords", { n: rules.minWords }) : t("ruleNoWords")} />
          <Rule ok={evidence.needFiles === 0} text={rules.minAttachments > 0 ? t("ruleFiles", { n: rules.minAttachments }) : t("ruleNoFiles")} />
          <Rule ok={!evidence.waiting} text={t("ruleReady")} />
        </ul>
        {error ? <FormError>{error}</FormError> : null}
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
          <Button onClick={send} disabled={!canSend} loading={pending} aria-describedby="proof-why">
            <PaperPlaneTilt aria-hidden weight="bold" className="size-4" />
            {pending ? t("sending") : editing ? t("update") : t("send")}
          </Button>
          <Button variant="ghost" asChild>
            <Link href="/today">{t("cancel")}</Link>
          </Button>
        </div>
        <p id="proof-why" role="status" className="text-sm text-muted">
          {reasons.length > 0 ? reasons.join(" ") : t("whyReady")}
        </p>
      </section>
    </div>
  );
}

function Rule({ ok, text }: { ok: boolean; text: string }) {
  const Icon = ok ? CheckCircle : Circle;
  return (
    <li className="flex items-center gap-2">
      <Icon aria-hidden weight={ok ? "fill" : "regular"} className={ok ? "size-4 text-credit" : "size-4 text-muted"} />
      <span className={ok ? "text-ink" : "text-muted"}>{text}</span>
    </li>
  );
}
