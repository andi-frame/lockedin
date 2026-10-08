import type { components } from "../api/schema";
import { addDays, daysBetween, isDate, isoWeekday, isTimeZone, todayIn } from "./dates";

// The wizard's working copy of a pact. Numbers stay strings while a person types, so "" and "1."
// are representable; `buildTerms` turns a validated draft into the API's Terms. The checks mirror
// SPEC §4 and `Terms.Validate` on the server, which stays the authority and re-checks everything.

type Terms = components["schemas"]["Terms"];
type MemberTerms = components["schemas"]["MemberTerms"];
type Pact = components["schemas"]["Pact"];
type PactDraft = components["schemas"]["PactDraft"];

/** The doer's slot in the terms until the invitee joins (SPEC §2, openapi `createPact`). */
export const NIL_ID = "00000000-0000-0000-0000-000000000000";

export type MemberDraft = {
  commitment: string;
  /** ISO weekdays, 1 = Monday. */
  schedule: number[];
  penalty: string;
  restDays: string;
  minAttachments: string;
  minWords: string;
};

export type Draft = {
  title: string;
  description: string;
  startsOn: string;
  endsOn: string;
  timezone: string;
  cutoff: string;
  graceMinutes: string;
  initialPot: string;
  /** "" means no cap. */
  potCap: string;
  reviewWindowHours: string;
  disputeWindowHours: string;
  disputeResolutionHours: string;
  overrideWindowHours: string;
  maxOverrides: string;
  backerCommits: boolean;
  backer: MemberDraft;
  doer: MemberDraft;
  // Not on the form yet. They ride along so editing a pact never changes what the person did not touch.
  version: number;
  coinRateIdr: number;
  potFloor: number;
};

export type Step = "basics" | "commitment" | "rules";
export const steps: readonly Step[] = ["basics", "commitment", "rules"];

/** Field path (`title`, `doer.schedule`) to a message key under `Wizard`. */
export type DraftErrors = Record<string, string>;

const workdays = [1, 2, 3, 4, 5];

const member = (minWords: string): MemberDraft => ({
  commitment: "",
  schedule: [...workdays],
  penalty: "50",
  restDays: "2",
  minAttachments: "1",
  minWords,
});

/** The SPEC §4 example, starting tomorrow (in the pact's zone) for thirty days. */
export function newDraft({ now, timezone }: { now: Date; timezone: string }): Draft {
  const startsOn = addDays(todayIn(timezone, now), 1);
  return {
    title: "",
    description: "",
    startsOn,
    endsOn: addDays(startsOn, 29),
    timezone,
    cutoff: "23:59",
    graceMinutes: "30",
    initialPot: "1000",
    potCap: "2000",
    reviewWindowHours: "24",
    disputeWindowHours: "24",
    disputeResolutionHours: "48",
    overrideWindowHours: "48",
    maxOverrides: "3",
    backerCommits: true,
    backer: member("0"),
    doer: member("20"),
    version: 1,
    coinRateIdr: 1000,
    potFloor: 0,
  };
}

/** A whole number written with digits only, or null. */
function whole(raw: string): number | null {
  const s = raw.trim();
  if (!/^\d+$/.test(s)) return null;
  const n = Number(s);
  return Number.isSafeInteger(n) ? n : null;
}

const inRange = (raw: string, min: number, max: number) => {
  const n = whole(raw);
  return n !== null && n >= min && n <= max;
};

const length = (s: string) => [...s.trim()].length;

function checkBasics(d: Draft, now: Date, e: DraftErrors) {
  const title = length(d.title);
  if (title === 0) e.title = "Wizard.titleRequired";
  else if (title > 120) e.title = "Wizard.titleLong";
  if (length(d.description) > 1000) e.description = "Wizard.descriptionLong";
  if (!isTimeZone(d.timezone)) e.timezone = "Wizard.timezoneInvalid";

  if (!isDate(d.startsOn)) e.startsOn = "Wizard.dateInvalid";
  else if (isTimeZone(d.timezone) && d.startsOn <= todayIn(d.timezone, now)) {
    // Check-ins are generated for every date from the start, and nothing stops a pact that starts in
    // the past from opening with missed days. Tomorrow is the earliest start that moves no coins.
    e.startsOn = "Wizard.startsPast";
  }

  if (!isDate(d.endsOn)) e.endsOn = "Wizard.dateInvalid";
  else if (isDate(d.startsOn)) {
    const days = daysBetween(d.startsOn, d.endsOn);
    if (days < 1) e.endsOn = "Wizard.endsBeforeStart";
    else if (days > 366) e.endsOn = "Wizard.durationLong";
  }
}

/** Whether any date in the range falls on one of the weekdays. */
function landsInRange(d: Draft, schedule: number[]): boolean {
  if (!isDate(d.startsOn) || !isDate(d.endsOn)) return true; // the basics step reports that
  const span = Math.min(daysBetween(d.startsOn, d.endsOn), 6);
  for (let i = 0; i <= span; i++) if (schedule.includes(isoWeekday(addDays(d.startsOn, i)))) return true;
  return false;
}

function checkMember(who: "backer" | "doer", d: Draft, e: DraftErrors) {
  const m = d[who];
  const key = (f: string) => `${who}.${f}`;
  const c = length(m.commitment);
  if (c === 0) e[key("commitment")] = "Wizard.commitmentRequired";
  else if (c > 200) e[key("commitment")] = "Wizard.commitmentLong";
  if (m.schedule.length === 0) e[key("schedule")] = "Wizard.scheduleRequired";
  else if (!landsInRange(d, m.schedule)) e[key("schedule")] = "Wizard.scheduleNoDates";
  if (!inRange(m.restDays, 0, 366)) e[key("restDays")] = "Wizard.restDaysInvalid";
  if (!inRange(m.minAttachments, 0, 10)) e[key("minAttachments")] = "Wizard.attachmentsRange";
  if (!inRange(m.minWords, 0, 2000)) e[key("minWords")] = "Wizard.wordsRange";
}

function checkRules(d: Draft, e: DraftErrors) {
  if (!/^([01]\d|2[0-3]):[0-5]\d$/.test(d.cutoff)) e.cutoff = "Wizard.cutoffInvalid";
  if (!inRange(d.graceMinutes, 0, 180)) e.graceMinutes = "Wizard.graceRange";
  const pot = whole(d.initialPot);
  if (pot === null || pot < 1) e.initialPot = "Wizard.potInvalid";
  else if (pot <= d.potFloor) e.initialPot = "Wizard.potBelowFloor";
  if (d.potCap.trim() !== "") {
    const cap = whole(d.potCap);
    if (cap === null) e.potCap = "Wizard.potInvalid";
    else if (pot !== null && cap < pot) e.potCap = "Wizard.capLow";
  }
  for (const f of ["reviewWindowHours", "disputeWindowHours", "disputeResolutionHours", "overrideWindowHours"] as const) {
    if (!inRange(d[f], 1, 72)) e[f] = "Wizard.hoursRange";
  }
  if (!inRange(d.maxOverrides, 0, 10)) e.maxOverrides = "Wizard.overridesRange";

  const penalised: ("backer" | "doer")[] = d.backerCommits ? ["backer", "doer"] : ["doer"];
  for (const who of penalised) {
    if (!inRange(d[who].penalty, 1, Number.MAX_SAFE_INTEGER)) e[`${who}.penalty`] = "Wizard.penaltyInvalid";
  }
}

export function validateStep(step: Step, d: Draft, now: Date): DraftErrors {
  const e: DraftErrors = {};
  if (step === "basics") checkBasics(d, now, e);
  else if (step === "commitment") {
    if (d.backerCommits) checkMember("backer", d, e);
    checkMember("doer", d, e);
  } else checkRules(d, e);
  return e;
}

/** The earliest step that still has a problem, so Propose can send the person back to it. */
export function firstInvalidStep(d: Draft, now: Date): { step: Step; errors: DraftErrors } | null {
  for (const step of steps) {
    const errors = validateStep(step, d, now);
    if (Object.keys(errors).length) return { step, errors };
  }
  return null;
}

const num = (raw: string) => Number(raw.trim());

function memberTerms(role: "backer" | "doer", m: MemberDraft, commits: boolean): MemberTerms {
  // A backer who does not commit has no schedule; the server ignores the rest of their rules.
  const schedule = commits ? [...m.schedule].sort((a, b) => a - b) : [];
  return {
    role,
    commitment: commits ? m.commitment.trim() : "",
    schedule,
    penalty_per_miss: num(m.penalty) >= 1 ? num(m.penalty) : 1,
    rest_days: commits ? num(m.restDays) : 0,
    evidence: commits
      ? { min_attachments: num(m.minAttachments), min_words: num(m.minWords) }
      : { min_attachments: 0, min_words: 0 },
  };
}

/** Terms for a draft that has passed `validateStep` on every step. */
export function buildTerms(d: Draft, ids: { backer: string; doer?: string }): Terms {
  return {
    version: d.version,
    timezone: d.timezone,
    starts_on: d.startsOn,
    ends_on: d.endsOn,
    cutoff_local_time: d.cutoff,
    grace_minutes: num(d.graceMinutes),
    coin_rate_idr: d.coinRateIdr,
    initial_pot: num(d.initialPot),
    pot_floor: d.potFloor,
    pot_cap: d.potCap.trim() === "" ? null : num(d.potCap),
    review_window_hours: num(d.reviewWindowHours),
    dispute_window_hours: num(d.disputeWindowHours),
    dispute_resolution_hours: num(d.disputeResolutionHours),
    override_window_hours: num(d.overrideWindowHours),
    max_overrides: num(d.maxOverrides),
    backer_commits: d.backerCommits,
    members: {
      [ids.backer]: memberTerms("backer", d.backer, d.backerCommits),
      [ids.doer ?? NIL_ID]: memberTerms("doer", d.doer, true),
    },
  };
}

/** The request body for `createPact` and `updatePact`. */
export function toPactDraft(d: Draft, ids: { backer: string; doer?: string }): PactDraft {
  const description = d.description.trim();
  return { title: d.title.trim(), description: description === "" ? null : description, terms: buildTerms(d, ids) };
}

const fromMember = (m: MemberTerms | undefined, fallback: MemberDraft): MemberDraft =>
  m
    ? {
        commitment: m.commitment,
        schedule: [...m.schedule],
        penalty: String(m.penalty_per_miss),
        restDays: String(m.rest_days),
        minAttachments: String(m.evidence.min_attachments),
        minWords: String(m.evidence.min_words),
      }
    : fallback;

/** Opens an existing pact in the wizard (edit while draft or proposed). */
export function draftFromPact(pact: Pact): Draft {
  const t = pact.terms;
  const base = newDraft({ now: new Date(0), timezone: t.timezone });
  const backer = t.members[pact.backer_id];
  const doerId = Object.keys(t.members).find((id) => id !== pact.backer_id);
  const doer = doerId ? t.members[doerId] : undefined;
  return {
    title: pact.title,
    description: pact.description ?? "",
    startsOn: t.starts_on,
    endsOn: t.ends_on,
    timezone: t.timezone,
    cutoff: t.cutoff_local_time,
    graceMinutes: String(t.grace_minutes),
    initialPot: String(t.initial_pot),
    potCap: t.pot_cap === null ? "" : String(t.pot_cap),
    reviewWindowHours: String(t.review_window_hours),
    disputeWindowHours: String(t.dispute_window_hours),
    disputeResolutionHours: String(t.dispute_resolution_hours),
    overrideWindowHours: String(t.override_window_hours),
    maxOverrides: String(t.max_overrides),
    backerCommits: t.backer_commits,
    // A backer who does not commit has blanks stored; turning commitment back on starts from the defaults.
    backer: t.backer_commits ? fromMember(backer, base.backer) : base.backer,
    doer: fromMember(doer, base.doer),
    version: t.version,
    coinRateIdr: t.coin_rate_idr,
    potFloor: t.pot_floor,
  };
}
