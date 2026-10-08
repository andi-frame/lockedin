"use client";

import { useTranslations } from "next-intl";
import type { ReactNode } from "react";
import { Field } from "@/components/ui/field";
import { Input, Textarea } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { formatIdr } from "@/lib/format";
import type { Draft, DraftErrors, MemberDraft } from "@/lib/pact/draft";
import { WeekdayPicker } from "./weekday-picker";

export type StepProps = {
  draft: Draft;
  errors: DraftErrors;
  /** The earliest date a pact may start, for the date pickers. */
  minStart: string;
  names: { backer: string; doer: string };
  set: (patch: Partial<Draft>) => void;
  setMember: (who: "backer" | "doer", patch: Partial<MemberDraft>) => void;
};

const zones = ["Asia/Jakarta", "Asia/Makassar", "Asia/Jayapura"] as const;

/** What a coin count is worth in Rupiah, only when the text is a whole number. */
function idrOf(raw: string, rate: number): string | undefined {
  const s = raw.trim();
  if (!/^\d+$/.test(s) || !Number.isSafeInteger(Number(s))) return undefined;
  return formatIdr(Number(s), rate);
}

function Group({ legend, children }: { legend: string; children: ReactNode }) {
  return (
    <fieldset className="flex min-w-0 flex-col gap-5 border-t border-rule pt-5">
      <legend className="-mb-1 pr-3 text-lg font-semibold tracking-tight">{legend}</legend>
      {children}
    </fieldset>
  );
}

const err = (t: (k: string) => string, errors: DraftErrors, path: string) => (errors[path] ? t(errors[path]) : undefined);

export function BasicsStep({ draft, errors, minStart, set }: StepProps) {
  const t = useTranslations();
  const e = (p: string) => err(t, errors, p);
  const knownZone = (zones as readonly string[]).includes(draft.timezone);
  return (
    <div className="flex flex-col gap-5">
      <Field label={t("Wizard.titleLabel")} hint={t("Wizard.titleHint")} error={e("title")}>
        <Input name="title" value={draft.title} maxLength={140} autoFocus onChange={(ev) => set({ title: ev.target.value })} />
      </Field>
      <Field label={t("Wizard.descriptionLabel")} hint={t("Wizard.descriptionHint")} error={e("description")}>
        <Textarea name="description" value={draft.description} rows={3} onChange={(ev) => set({ description: ev.target.value })} />
      </Field>
      <div className="grid gap-5 sm:grid-cols-2">
        <Field label={t("Wizard.startsLabel")} error={e("startsOn")}>
          <Input name="startsOn" type="date" min={minStart} value={draft.startsOn} onChange={(ev) => set({ startsOn: ev.target.value })} />
        </Field>
        <Field label={t("Wizard.endsLabel")} error={e("endsOn")}>
          <Input name="endsOn" type="date" min={draft.startsOn} value={draft.endsOn} onChange={(ev) => set({ endsOn: ev.target.value })} />
        </Field>
      </div>
      <Field label={t("Wizard.timezoneLabel")} hint={t("Wizard.timezoneHint")} error={e("timezone")}>
        <Select value={draft.timezone} onValueChange={(timezone) => set({ timezone })}>
          <SelectTrigger name="timezone">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {zones.map((z) => (
              <SelectItem key={z} value={z}>
                {t(`Wizard.zone_${z.split("/")[1]}`)}
              </SelectItem>
            ))}
            {knownZone ? null : <SelectItem value={draft.timezone}>{draft.timezone}</SelectItem>}
          </SelectContent>
        </Select>
      </Field>
    </div>
  );
}

function MemberFields({ who, props }: { who: "backer" | "doer"; props: StepProps }) {
  const t = useTranslations();
  const { draft, errors, setMember } = props;
  const m = draft[who];
  const e = (f: string) => err(t, errors, `${who}.${f}`);
  const num = (f: "restDays" | "minAttachments" | "minWords", label: string, hint?: string) => (
    <Field label={label} hint={hint} error={e(f)}>
      <Input name={`${who}.${f}`} inputMode="numeric" value={m[f]} onChange={(ev) => setMember(who, { [f]: ev.target.value })} />
    </Field>
  );
  return (
    <div className="flex flex-col gap-5">
      <Field label={t("Wizard.commitmentLabel")} hint={t(`Wizard.commitmentHint_${who}`)} error={e("commitment")}>
        <Textarea
          name={`${who}.commitment`}
          value={m.commitment}
          rows={2}
          maxLength={220}
          className="min-h-20"
          onChange={(ev) => setMember(who, { commitment: ev.target.value })}
        />
      </Field>
      <WeekdayPicker
        name={`${who}.schedule`}
        legend={t("Wizard.scheduleLabel")}
        value={m.schedule}
        error={e("schedule")}
        onChange={(schedule) => setMember(who, { schedule })}
      />
      <div className="grid gap-5 sm:grid-cols-3">
        {num("minAttachments", t("Wizard.minAttachmentsLabel"))}
        {num("minWords", t("Wizard.minWordsLabel"))}
        {num("restDays", t("Wizard.restDaysLabel"), t("Wizard.restDaysHint"))}
      </div>
    </div>
  );
}

export function CommitmentStep(props: StepProps) {
  const t = useTranslations();
  const { draft, names, set } = props;
  return (
    <div className="flex flex-col gap-6">
      <Group legend={t("Wizard.groupBacker", { name: names.backer })}>
        <label className="flex min-h-11 cursor-pointer items-start gap-3 text-[15px]">
          <input
            type="checkbox"
            name="backerCommits"
            checked={draft.backerCommits}
            onChange={(ev) => set({ backerCommits: ev.target.checked })}
            className="mt-1 size-5 shrink-0 accent-primary"
          />
          <span>
            <span className="block font-medium">{t("Wizard.backerCommitsLabel")}</span>
            <span className="block text-sm text-muted">{t("Wizard.backerCommitsHint")}</span>
          </span>
        </label>
        {draft.backerCommits ? <MemberFields who="backer" props={props} /> : null}
      </Group>
      <Group legend={t("Wizard.groupDoer")}>
        <MemberFields who="doer" props={props} />
      </Group>
    </div>
  );
}

export function RulesStep(props: StepProps) {
  const t = useTranslations();
  const { draft, errors, set, setMember } = props;
  const e = (p: string) => err(t, errors, p);
  const rate = draft.coinRateIdr;
  const field = (name: keyof Draft & string, label: string, opts: { hint?: string; idr?: boolean; optional?: boolean } = {}) => {
    const value = draft[name] as string;
    const idr = opts.idr ? idrOf(value, rate) : undefined;
    return (
      <Field label={label} hint={opts.hint} error={e(name)}>
        <Input name={name} inputMode="numeric" value={value} onChange={(ev) => set({ [name]: ev.target.value })} />
        {idr ? <p className="font-mono text-sm tabular-nums text-muted">{t("Wizard.worth", { idr })}</p> : null}
      </Field>
    );
  };
  const penalty = (who: "backer" | "doer", label: string, hint: string) => {
    const value = draft[who].penalty;
    const idr = idrOf(value, rate);
    return (
      <Field label={label} hint={hint} error={e(`${who}.penalty`)}>
        <Input name={`${who}.penalty`} inputMode="numeric" value={value} onChange={(ev) => setMember(who, { penalty: ev.target.value })} />
        {idr ? <p className="font-mono text-sm tabular-nums text-muted">{t("Wizard.worth", { idr })}</p> : null}
      </Field>
    );
  };
  return (
    <div className="flex flex-col gap-6">
      <Group legend={t("Wizard.groupCoins")}>
        <div className="grid gap-5 sm:grid-cols-2">
          {field("initialPot", t("Wizard.potLabel"), { hint: t("Wizard.potHint"), idr: true })}
          {field("potCap", t("Wizard.capLabel"), { hint: t("Wizard.capHint"), idr: true })}
          {penalty("doer", t("Wizard.penaltyDoerLabel"), t("Wizard.penaltyDoerHint"))}
          {draft.backerCommits ? penalty("backer", t("Wizard.penaltyBackerLabel"), t("Wizard.penaltyBackerHint")) : null}
        </div>
      </Group>
      <Group legend={t("Wizard.groupDeadline")}>
        <div className="grid gap-5 sm:grid-cols-2">
          <Field label={t("Wizard.cutoffLabel")} hint={t("Wizard.cutoffHint")} error={e("cutoff")}>
            <Input name="cutoff" type="time" value={draft.cutoff} onChange={(ev) => set({ cutoff: ev.target.value })} />
          </Field>
          {field("graceMinutes", t("Wizard.graceLabel"), { hint: t("Wizard.graceHint") })}
        </div>
      </Group>
      <Group legend={t("Wizard.groupReview")}>
        <div className="grid gap-5 sm:grid-cols-2">
          {field("reviewWindowHours", t("Wizard.reviewLabel"), { hint: t("Wizard.reviewHint") })}
          {field("disputeWindowHours", t("Wizard.disputeLabel"), { hint: t("Wizard.disputeHint") })}
          {field("disputeResolutionHours", t("Wizard.resolutionLabel"), { hint: t("Wizard.resolutionHint") })}
          {field("maxOverrides", t("Wizard.overridesLabel"), { hint: t("Wizard.overridesHint") })}
          {field("overrideWindowHours", t("Wizard.overrideWindowLabel"), { hint: t("Wizard.overrideWindowHint") })}
        </div>
      </Group>
    </div>
  );
}
