import { describe, expect, test } from "bun:test";
import type { components } from "../api/schema";
import {
  NIL_ID,
  buildTerms,
  draftFromPact,
  firstInvalidStep,
  newDraft,
  toPactDraft,
  validateStep,
  type Draft,
} from "./draft";

type Pact = components["schemas"]["Pact"];

// 2026-10-08 12:00 in Jakarta. Tomorrow there is 2026-10-09, a Friday.
const now = new Date("2026-10-08T05:00:00Z");
const BACKER = "11111111-1111-4111-8111-111111111111";
const DOER = "22222222-2222-4222-8222-222222222222";

function filled(over: Partial<Draft> = {}): Draft {
  return {
    ...newDraft({ now, timezone: "Asia/Jakarta" }),
    title: "Kalkulus sebulan",
    ...over,
  };
}
const withMember = (d: Draft, who: "backer" | "doer", over: Partial<Draft["backer"]>): Draft => ({
  ...d,
  [who]: { ...d[who], ...over },
});

describe("newDraft", () => {
  test("starts tomorrow in the pact zone and runs thirty days", () => {
    const d = newDraft({ now, timezone: "Asia/Jakarta" });
    expect(d.startsOn).toBe("2026-10-09");
    expect(d.endsOn).toBe("2026-11-07");
    expect(d.timezone).toBe("Asia/Jakarta");
  });
  test("tomorrow follows the zone, not the instant", () => {
    // 20:00 UTC on the 8th is already the 9th in Jakarta.
    const late = new Date("2026-10-08T20:00:00Z");
    expect(newDraft({ now: late, timezone: "Asia/Jakarta" }).startsOn).toBe("2026-10-10");
    expect(newDraft({ now: late, timezone: "UTC" }).startsOn).toBe("2026-10-09");
  });
  test("a fresh draft only lacks the words only the person can write", () => {
    expect(firstInvalidStep(filled(), now)).toEqual({ step: "commitment", errors: expect.any(Object) });
    const ok = withMember(withMember(filled(), "backer", { commitment: "Belajar" }), "doer", { commitment: "Latihan" });
    expect(firstInvalidStep(ok, now)).toBeNull();
  });
});

describe("validateStep basics", () => {
  test.each<[string, Partial<Draft>, string]>([
    ["empty title", { title: "   " }, "title"],
    ["title over 120", { title: "x".repeat(121) }, "title"],
    ["description over 1000", { description: "x".repeat(1001) }, "description"],
    ["start today", { startsOn: "2026-10-08" }, "startsOn"],
    ["start in the past", { startsOn: "2026-10-01" }, "startsOn"],
    ["start not a date", { startsOn: "2026-02-30" }, "startsOn"],
    ["end not after start", { startsOn: "2026-10-20", endsOn: "2026-10-20" }, "endsOn"],
    ["end before start", { startsOn: "2026-10-20", endsOn: "2026-10-10" }, "endsOn"],
    ["longer than 366 days", { startsOn: "2026-10-09", endsOn: "2027-10-12" }, "endsOn"],
    ["unknown zone", { timezone: "Mars/Olympus" }, "timezone"],
  ])("%s is refused on %s", (_name, over, field) => {
    expect(validateStep("basics", filled(over), now)[field]).toMatch(/^Wizard\./);
  });

  test("accepts tomorrow, one day, and exactly 366 days", () => {
    expect(validateStep("basics", filled({ startsOn: "2026-10-09", endsOn: "2026-10-10" }), now)).toEqual({});
    expect(validateStep("basics", filled({ startsOn: "2026-10-09", endsOn: "2027-10-10" }), now)).toEqual({});
  });

  test("today is judged in the pact's zone", () => {
    // In Los Angeles it is still the morning of 2026-10-07 at this instant, so the 8th is fine there.
    expect(validateStep("basics", filled({ timezone: "America/Los_Angeles", startsOn: "2026-10-08" }), now)).toEqual({});
  });
});

describe("validateStep commitment", () => {
  const base = () => withMember(withMember(filled(), "backer", { commitment: "Belajar" }), "doer", { commitment: "Latihan" });

  test("a complete draft passes", () => expect(validateStep("commitment", base(), now)).toEqual({}));

  test.each<[string, "backer" | "doer", Partial<Draft["backer"]>, string]>([
    ["blank commitment", "doer", { commitment: "  " }, "doer.commitment"],
    ["commitment over 200", "doer", { commitment: "x".repeat(201) }, "doer.commitment"],
    ["no weekdays", "doer", { schedule: [] }, "doer.schedule"],
    ["attachments over 10", "doer", { minAttachments: "11" }, "doer.minAttachments"],
    ["attachments not a number", "doer", { minAttachments: "dua" }, "doer.minAttachments"],
    ["words over 2000", "doer", { minWords: "2001" }, "doer.minWords"],
    ["negative rest days", "doer", { restDays: "-1" }, "doer.restDays"],
    ["backer blank commitment", "backer", { commitment: "" }, "backer.commitment"],
  ])("%s", (_name, who, over, field) => {
    expect(validateStep("commitment", withMember(base(), who, over), now)[field]).toMatch(/^Wizard\./);
  });

  test("a schedule that never lands inside the dates is refused", () => {
    // 2026-10-09 (Fri) to 2026-10-10 (Sat): a Monday-only schedule gets no check-ins.
    const d = withMember(base(), "doer", { schedule: [1] });
    d.startsOn = "2026-10-09";
    d.endsOn = "2026-10-10";
    expect(validateStep("commitment", d, now)["doer.schedule"]).toBe("Wizard.scheduleNoDates");
  });

  test("the backer's own rules only matter when the backer commits", () => {
    const off = { ...withMember(base(), "backer", { commitment: "", schedule: [], minWords: "x" }), backerCommits: false };
    expect(validateStep("commitment", off, now)).toEqual({});
  });
});

describe("validateStep rules", () => {
  test("defaults pass", () => expect(validateStep("rules", filled(), now)).toEqual({}));

  test.each<[string, Partial<Draft>, string]>([
    ["cutoff not HH:MM", { cutoff: "24:00" }, "cutoff"],
    ["cutoff blank", { cutoff: "" }, "cutoff"],
    ["grace over 180", { graceMinutes: "181" }, "graceMinutes"],
    ["pot of zero", { initialPot: "0" }, "initialPot"],
    ["pot with a decimal", { initialPot: "10.5" }, "initialPot"],
    ["cap below the pot", { initialPot: "1000", potCap: "999" }, "potCap"],
    ["review window of zero", { reviewWindowHours: "0" }, "reviewWindowHours"],
    ["dispute window over 72", { disputeWindowHours: "73" }, "disputeWindowHours"],
    ["resolution window over 72", { disputeResolutionHours: "73" }, "disputeResolutionHours"],
    ["override window over 72", { overrideWindowHours: "73" }, "overrideWindowHours"],
    ["overrides over 10", { maxOverrides: "11" }, "maxOverrides"],
  ])("%s", (_name, over, field) => {
    expect(validateStep("rules", filled(over), now)[field]).toMatch(/^Wizard\./);
  });

  test("a cap of nothing means no cap, and zero overrides turns the power off", () => {
    expect(validateStep("rules", filled({ potCap: "", maxOverrides: "0" }), now)).toEqual({});
  });

  test("penalty must be at least 1 for the doer, and for the backer only when committing", () => {
    expect(validateStep("rules", withMember(filled(), "doer", { penalty: "0" }), now)["doer.penalty"]).toMatch(/^Wizard\./);
    const backerZero = withMember(filled(), "backer", { penalty: "0" });
    expect(validateStep("rules", backerZero, now)["backer.penalty"]).toMatch(/^Wizard\./);
    expect(validateStep("rules", { ...backerZero, backerCommits: false }, now)).toEqual({});
  });
});

describe("firstInvalidStep", () => {
  test("reports the earliest step with a problem", () => {
    const d = filled({ title: "", initialPot: "0" });
    expect(firstInvalidStep(d, now)?.step).toBe("basics");
  });
});

describe("buildTerms", () => {
  const d = withMember(
    withMember(filled({ potCap: "2000", cutoff: "21:30" }), "backer", { commitment: " Belajar Kalkulus ", schedule: [3, 1, 2] }),
    "doer",
    { commitment: "Latihan soal", penalty: "75", minWords: "20" },
  );

  test("keys the backer by id and the doer slot by the nil UUID until someone joins", () => {
    const t = buildTerms(d, { backer: BACKER });
    expect(Object.keys(t.members).sort()).toEqual([NIL_ID, BACKER].sort());
    expect(t.members[BACKER]?.role).toBe("backer");
    expect(t.members[NIL_ID]?.role).toBe("doer");
  });

  test("uses the joined doer's id on edit", () => {
    const t = buildTerms(d, { backer: BACKER, doer: DOER });
    expect(Object.keys(t.members).sort()).toEqual([BACKER, DOER].sort());
  });

  test("numbers are integers, text is trimmed, weekdays are sorted", () => {
    const t = buildTerms(d, { backer: BACKER });
    expect(t.members[BACKER]?.commitment).toBe("Belajar Kalkulus");
    expect(t.members[BACKER]?.schedule).toEqual([1, 2, 3]);
    expect(t.members[NIL_ID]?.penalty_per_miss).toBe(75);
    expect(t.members[NIL_ID]?.evidence).toEqual({ min_attachments: 1, min_words: 20 });
    expect(t).toMatchObject({
      version: 1,
      timezone: "Asia/Jakarta",
      starts_on: "2026-10-09",
      cutoff_local_time: "21:30",
      grace_minutes: 30,
      coin_rate_idr: 1000,
      initial_pot: 1000,
      pot_floor: 0,
      pot_cap: 2000,
      backer_commits: true,
    });
  });

  test("no cap becomes null, not zero", () => {
    expect(buildTerms({ ...d, potCap: "" }, { backer: BACKER }).pot_cap).toBeNull();
  });

  test("a backer who does not commit has an empty schedule", () => {
    const t = buildTerms({ ...d, backerCommits: false }, { backer: BACKER });
    expect(t.backer_commits).toBe(false);
    expect(t.members[BACKER]?.schedule).toEqual([]);
  });
});

describe("draftFromPact", () => {
  const pact = (): Pact => {
    const terms = buildTerms(
      withMember(withMember(filled({ potCap: "" }), "backer", { commitment: "B" }), "doer", { commitment: "D", penalty: "20" }),
      { backer: BACKER, doer: DOER },
    );
    return { id: "p", title: "Judul", description: null, terms, backer_id: BACKER } as unknown as Pact;
  };

  test("round trips through buildTerms", () => {
    const p = pact();
    const draft = draftFromPact(p);
    expect(draft.title).toBe("Judul");
    expect(draft.doer.penalty).toBe("20");
    expect(draft.potCap).toBe("");
    expect(buildTerms(draft, { backer: BACKER, doer: DOER })).toEqual(p.terms);
  });

  test("toPactDraft is the PATCH/POST body", () => {
    const body = toPactDraft({ ...draftFromPact(pact()), description: "  " }, { backer: BACKER, doer: DOER });
    expect(body.title).toBe("Judul");
    expect(body.description).toBeNull(); // blank means none
    expect(body.terms.members[DOER]?.role).toBe("doer");
  });
});
