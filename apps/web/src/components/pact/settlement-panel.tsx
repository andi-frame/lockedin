import { CheckCircle, Circle } from "@phosphor-icons/react/dist/ssr";
import { useFormatter, useTranslations } from "next-intl";
import { Amount } from "@/components/amount";
import type { components } from "@/lib/api/schema";
import { settlementState } from "@/lib/pact/settlement";
import { ConfirmReceiptButton, MarkPaidButton } from "./payout-actions";

type Pact = components["schemas"]["Pact"];

/**
 * What is owed once the pact has settled, and who has done what about it (SPEC §3): the amount is
 * the payout the worker fixed, the backer says it was paid, and the doer's confirmation completes
 * the pact. The app keeps the record only; no money moves through it.
 */
export function SettlementPanel({ pact, timeZone }: { pact: Pact; timeZone: string }) {
  const t = useTranslations("Settlement");
  const f = useFormatter();
  const s = settlementState(pact);
  if (!s) return null;

  const backer = pact.members.find((m) => m.role === "backer")?.display_name ?? "";
  const doer = pact.members.find((m) => m.role === "doer")?.display_name ?? "";
  const when = (iso: string | null) => (iso ? f.dateTime(new Date(iso), { dateStyle: "medium", timeStyle: "short", timeZone }) : "");
  const completed = s.phase === "completed";

  return (
    <section aria-labelledby="settlement" className="mt-8 max-w-3xl rounded-panel border border-rule-strong bg-surface p-5 sm:p-6" data-testid="settlement" data-phase={s.phase}>
      <h2 id="settlement" className="text-lg font-semibold tracking-tight">
        {completed ? t("titleCompleted") : t("titleSettling")}
      </h2>

      {s.owes ? (
        <div className="mt-3">
          <p className="text-sm text-muted">{t("owedLabel")}</p>
          <Amount coins={s.amount} direction="balance" rate={pact.terms.coin_rate_idr} size="xl" />
        </div>
      ) : null}
      <p className="mt-3 max-w-prose text-[15px]">{completed ? t("completedBody") : s.owes ? t("owedBody", { backer, doer }) : t("nothingOwed")}</p>

      <ul className="mt-4 flex flex-col gap-2 text-[15px]">
        <Step done={s.markedPaid} yes={t("paidStep", { name: backer })} no={t("notPaid", { name: backer })} when={when(s.markedPaidAt)} />
        {s.note ? <li className="ml-8 text-sm text-muted [overflow-wrap:anywhere]">{t("note", { note: s.note })}</li> : null}
        <Step done={s.confirmed} yes={t("confirmedStep", { name: doer })} no={t("notConfirmed", { name: doer })} when={when(s.confirmedAt)} />
      </ul>

      {s.canMarkPaid || s.canConfirm ? (
        <div className="mt-5 flex flex-wrap gap-3">
          {s.canMarkPaid ? <MarkPaidButton pactId={pact.id} doerName={doer} /> : null}
          {s.canConfirm ? <ConfirmReceiptButton pactId={pact.id} backerMarkedPaid={s.markedPaid} /> : null}
        </div>
      ) : null}
      {!completed && pact.my_role === "doer" && !s.markedPaid && s.owes ? <p className="mt-3 text-sm text-muted">{t("waitingBacker")}</p> : null}
      {!completed && pact.my_role === "backer" && s.markedPaid ? <p className="mt-3 text-sm text-muted">{t("waitingDoer", { name: doer })}</p> : null}
    </section>
  );
}

function Step({ done, yes, no, when }: { done: boolean; yes: string; no: string; when: string }) {
  return (
    <li className="flex items-start gap-3" data-done={done}>
      {done ? <CheckCircle aria-hidden weight="fill" className="mt-0.5 size-5 shrink-0 text-teal-text" /> : <Circle aria-hidden className="mt-0.5 size-5 shrink-0 text-muted" />}
      <span className="min-w-0">
        <span className={done ? "font-medium" : "text-muted"}>{done ? yes : no}</span>
        {done && when ? <span className="ml-2 font-mono text-[13px] tabular-nums text-muted">{when}</span> : null}
      </span>
    </li>
  );
}
