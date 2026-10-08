import { describe, expect, test } from "bun:test";
import en from "../../../messages/en.json";
import id from "../../../messages/id.json";
import type { components } from "../api/schema";
import { summaryKeys, termsSummary, type SummaryLine } from "./summary";

type Terms = components["schemas"]["Terms"];
type MemberTerms = components["schemas"]["MemberTerms"];

const B = "11111111-1111-4111-8111-111111111111";
const D = "22222222-2222-4222-8222-222222222222";

const terms = (over: Partial<Terms> = {}): Terms => ({
  version: 1,
  timezone: "Asia/Jakarta",
  starts_on: "2026-11-01",
  ends_on: "2026-11-30",
  cutoff_local_time: "23:59",
  grace_minutes: 30,
  coin_rate_idr: 1000,
  initial_pot: 1000,
  pot_floor: 0,
  pot_cap: 2000,
  review_window_hours: 24,
  dispute_window_hours: 24,
  dispute_resolution_hours: 48,
  override_window_hours: 48,
  max_overrides: 3,
  backer_commits: true,
  members: {
    [B]: { role: "backer", commitment: "Belajar Kalkulus 2 jam", schedule: [1, 2, 3, 4, 5], penalty_per_miss: 50, rest_days: 2, evidence: { min_attachments: 1, min_words: 0 } },
    [D]: { role: "doer", commitment: "Latihan soal UTBK", schedule: [1, 2, 3, 4, 5, 6], penalty_per_miss: 75, rest_days: 1, evidence: { min_attachments: 1, min_words: 20 } },
  },
  ...over,
});

const member = (t: Terms, id: string): MemberTerms => {
  const m = t.members[id];
  if (!m) throw new Error(`no member ${id}`);
  return m;
};
const names = { backer: "Andi", doer: "Sari" };
const lines = (t: Terms) => termsSummary(t, { locale: "id", names });
const find = (ls: SummaryLine[], key: string) => ls.filter((l) => l.key === key);
const one = (ls: SummaryLine[], key: string) => {
  const hits = find(ls, key);
  expect(hits).toHaveLength(1);
  return hits[0]?.params ?? {};
};

describe("termsSummary", () => {
  test("states the doer's penalty in coins and Rupiah, the example SPEC §4 gives", () => {
    expect(one(lines(terms()), "Terms.doerMiss")).toMatchObject({ name: "Sari", coins: "75", idr: "Rp75.000" });
  });

  test("states what the backer adds when the backer misses, with the cap", () => {
    expect(one(lines(terms()), "Terms.backerMiss")).toMatchObject({ name: "Andi", coins: "50", idr: "Rp50.000", cap: "2.000" });
  });

  test("with no cap the backer's line says so", () => {
    expect(find(lines(terms({ pot_cap: null })), "Terms.backerMissNoCap")).toHaveLength(1);
    expect(find(lines(terms({ pot_cap: null })), "Terms.backerMiss")).toHaveLength(0);
  });

  test("a backer who does not commit has no miss line and no cap line", () => {
    const t = terms({ backer_commits: false });
    t.members[B] = { ...member(t, B), schedule: [], commitment: "" };
    const ls = lines(t);
    expect(find(ls, "Terms.backerMiss")).toHaveLength(0);
    expect(find(ls, "Terms.backerMissNoCap")).toHaveLength(0);
    expect(find(ls, "Terms.backerNoCommitment")).toHaveLength(1);
    expect(find(ls, "Terms.commitment")).toHaveLength(1); // only the doer's
  });

  test("the pot line carries the starting pot and the floor", () => {
    expect(one(lines(terms()), "Terms.pot")).toMatchObject({ coins: "1.000", idr: "Rp1.000.000", floor: "0" });
  });

  test("the period counts days inclusively and names the zone", () => {
    expect(one(lines(terms()), "Terms.period")).toMatchObject({ days: 30, tz: "Asia/Jakarta" });
  });

  test("commitments list the weekdays in the reader's language", () => {
    const doer = find(lines(terms()), "Terms.commitment").find((l) => l.params.name === "Sari");
    expect(doer?.params.days).toBe("Senin, Selasa, Rabu, Kamis, Jumat, dan Sabtu");
    const en = termsSummary(terms(), { locale: "en", names }).find((l) => l.key === "Terms.commitment" && l.params.name === "Sari");
    expect(en?.params.days).toBe("Monday, Tuesday, Wednesday, Thursday, Friday, and Saturday");
  });

  test.each<[string, { min_attachments: number; min_words: number }, string]>([
    ["both", { min_attachments: 2, min_words: 30 }, "Terms.evidenceBoth"],
    ["files only", { min_attachments: 1, min_words: 0 }, "Terms.evidenceFiles"],
    ["words only", { min_attachments: 0, min_words: 15 }, "Terms.evidenceWords"],
    ["nothing", { min_attachments: 0, min_words: 0 }, "Terms.evidenceNone"],
  ])("evidence rule: %s", (_n, evidence, key) => {
    const t = terms();
    t.members[D] = { ...member(t, D), evidence };
    const hit = find(lines(t), key).find((l) => l.params.name === "Sari");
    expect(hit).toBeDefined();
  });

  test("the cutoff line mentions grace only when there is some", () => {
    expect(find(lines(terms()), "Terms.cutoff")).toHaveLength(1);
    expect(find(lines(terms({ grace_minutes: 0 })), "Terms.cutoffNoGrace")).toHaveLength(1);
  });

  test("zero rest days says there are none", () => {
    const t = terms();
    t.members[D] = { ...member(t, D), rest_days: 0 };
    expect(find(lines(t), "Terms.restNone").map((l) => l.params.name)).toEqual(["Sari"]);
  });

  test("override power is described only when it exists, and it is visible to both", () => {
    expect(one(lines(terms()), "Terms.override")).toMatchObject({ backer: "Andi", doer: "Sari", hours: 48, max: 3 });
    expect(find(lines(terms({ max_overrides: 0 })), "Terms.overrideOff")).toHaveLength(1);
    expect(find(lines(terms({ max_overrides: 0 })), "Terms.override")).toHaveLength(0);
  });

  test("always ends with the IOU line: no real money moves", () => {
    const ls = lines(terms());
    expect(ls.at(-1)?.key).toBe("Terms.iou");
  });

  test("a large pot is exact in Rupiah", () => {
    const t = terms({ initial_pot: 9_007_199_254, coin_rate_idr: 1000, pot_cap: null });
    expect(one(lines(t), "Terms.pot").idr).toBe("Rp9.007.199.254.000");
  });
});

describe("copy", () => {
  const flat = (obj: Record<string, string>) => new Set(Object.keys(obj));
  test.each([
    ["id", id.Terms],
    ["en", en.Terms],
  ])("%s has a message for every summary line", (_l, messages) => {
    const have = flat(messages as Record<string, string>);
    for (const key of summaryKeys) expect(have.has(key.replace("Terms.", ""))).toBe(true);
  });
});

describe("copy placeholders", () => {
  // A typo like {coin} for {coins} would print the raw placeholder to the people signing.
  const placeholders = (s: string) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1] ?? "");
  const variants: Terms[] = [
    terms(),
    terms({ pot_cap: null, grace_minutes: 0, max_overrides: 0 }),
    terms({ backer_commits: false }),
  ];
  test.each([
    ["id", id.Terms],
    ["en", en.Terms],
  ])("%s uses only the parameters the line provides", (locale, messages) => {
    for (const t of variants) {
      for (const line of termsSummary(t, { locale: locale as "id" | "en", names })) {
        const text = (messages as Record<string, string>)[line.key.replace("Terms.", "")] ?? "";
        for (const p of placeholders(text)) expect(Object.keys(line.params)).toContain(p);
      }
    }
  });
});
