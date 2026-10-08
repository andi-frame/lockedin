import type { components } from "../api/schema";
import { formatCoins, formatIdr } from "../format";
import { addDays, daysBetween } from "./dates";

// The plain-language terms summary SPEC §4 requires before anyone signs. It is built from the same
// Terms the hash covers, so what is read is what is signed. This file decides which sentences
// apply and with what numbers; the wording lives in messages/*.json under `Terms`.

type Terms = components["schemas"]["Terms"];
type MemberTerms = components["schemas"]["MemberTerms"];

export type SummaryLine = { key: `Terms.${string}`; params: Record<string, string | number> };

/** Every key `termsSummary` can emit. A test checks that both message files have all of them. */
export const summaryKeys = [
  "Terms.period",
  "Terms.commitment",
  "Terms.backerNoCommitment",
  "Terms.cutoff",
  "Terms.cutoffNoGrace",
  "Terms.evidenceBoth",
  "Terms.evidenceFiles",
  "Terms.evidenceWords",
  "Terms.evidenceNone",
  "Terms.pot",
  "Terms.doerMiss",
  "Terms.backerMiss",
  "Terms.backerMissNoCap",
  "Terms.rest",
  "Terms.restNone",
  "Terms.reviewOneWay",
  "Terms.reviewBoth",
  "Terms.dispute",
  "Terms.override",
  "Terms.overrideOff",
  "Terms.iou",
] as const;

export type SummaryOptions = {
  locale: "id" | "en";
  /** How each member is named in the sentences ("kamu" when the reader is that member). */
  names: { backer: string; doer: string };
};

const intlLocale = (l: SummaryOptions["locale"]) => (l === "id" ? "id-ID" : "en-US");

function weekdays(days: number[], locale: SummaryOptions["locale"]): string {
  const monday = "2026-11-02"; // a known Monday
  const name = new Intl.DateTimeFormat(intlLocale(locale), { weekday: "long", timeZone: "UTC" });
  const sorted = [...days].sort((a, b) => a - b);
  const names = sorted.map((d) => name.format(new Date(`${addDays(monday, d - 1)}T00:00:00Z`)));
  return new Intl.ListFormat(intlLocale(locale), { style: "long", type: "conjunction" }).format(names);
}

function longDate(iso: string, locale: SummaryOptions["locale"]): string {
  return new Intl.DateTimeFormat(intlLocale(locale), { dateStyle: "long", timeZone: "UTC" }).format(new Date(`${iso}T00:00:00Z`));
}

export function termsSummary(terms: Terms, { locale, names }: SummaryOptions): SummaryLine[] {
  const rate = terms.coin_rate_idr;
  const money = (coins: number) => ({ coins: formatCoins(coins), idr: formatIdr(coins, rate) });
  const byRole = (role: "backer" | "doer"): MemberTerms | undefined => Object.values(terms.members).find((m) => m.role === role);
  const backer = byRole("backer");
  const doer = byRole("doer");
  const out: SummaryLine[] = [];

  out.push({
    key: "Terms.period",
    params: {
      start: longDate(terms.starts_on, locale),
      end: longDate(terms.ends_on, locale),
      days: daysBetween(terms.starts_on, terms.ends_on) + 1,
      tz: terms.timezone,
    },
  });

  const commits: { who: "backer" | "doer"; m: MemberTerms }[] = [];
  if (backer && terms.backer_commits) commits.push({ who: "backer", m: backer });
  if (doer) commits.push({ who: "doer", m: doer });

  if (!terms.backer_commits) out.push({ key: "Terms.backerNoCommitment", params: { name: names.backer } });
  for (const { who, m } of commits) {
    out.push({ key: "Terms.commitment", params: { name: names[who], what: m.commitment, days: weekdays(m.schedule, locale) } });
  }

  out.push(
    terms.grace_minutes > 0
      ? { key: "Terms.cutoff", params: { time: terms.cutoff_local_time, tz: terms.timezone, grace: terms.grace_minutes } }
      : { key: "Terms.cutoffNoGrace", params: { time: terms.cutoff_local_time, tz: terms.timezone } },
  );

  for (const { who, m } of commits) {
    const { min_attachments: files, min_words: words } = m.evidence;
    const key = files && words ? "Terms.evidenceBoth" : files ? "Terms.evidenceFiles" : words ? "Terms.evidenceWords" : "Terms.evidenceNone";
    out.push({ key, params: { name: names[who], files, words } });
  }

  out.push({ key: "Terms.pot", params: { ...money(terms.initial_pot), floor: formatCoins(terms.pot_floor) } });
  if (doer) out.push({ key: "Terms.doerMiss", params: { name: names.doer, ...money(doer.penalty_per_miss) } });
  if (backer && terms.backer_commits) {
    const miss = money(backer.penalty_per_miss);
    out.push(
      terms.pot_cap === null
        ? { key: "Terms.backerMissNoCap", params: { name: names.backer, ...miss } }
        : { key: "Terms.backerMiss", params: { name: names.backer, ...miss, cap: formatCoins(terms.pot_cap) } },
    );
  }

  for (const { who, m } of commits) {
    out.push(
      m.rest_days > 0
        ? { key: "Terms.rest", params: { name: names[who], n: m.rest_days } }
        : { key: "Terms.restNone", params: { name: names[who] } },
    );
  }

  out.push({
    key: terms.backer_commits ? "Terms.reviewBoth" : "Terms.reviewOneWay",
    params: { backer: names.backer, doer: names.doer, hours: terms.review_window_hours },
  });
  out.push({
    key: "Terms.dispute",
    params: { hours: terms.dispute_window_hours, resolution: terms.dispute_resolution_hours },
  });
  out.push(
    terms.max_overrides > 0
      ? { key: "Terms.override", params: { backer: names.backer, doer: names.doer, hours: terms.override_window_hours, max: terms.max_overrides } }
      : { key: "Terms.overrideOff", params: { backer: names.backer } },
  );

  out.push({ key: "Terms.iou", params: {} });
  return out;
}
