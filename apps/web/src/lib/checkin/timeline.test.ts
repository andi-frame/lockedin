import { describe, expect, test } from "bun:test";
import type { components } from "../api/schema";
import { nextDeadline, overrideDecision, timelineEntries } from "./timeline";

type Decision = components["schemas"]["Decision"];
type CheckIn = components["schemas"]["CheckIn"];

const ME = "me";
const THEM = "them";
const names = { [ME]: "Bima", [THEM]: "Andi" };
let n = 0;
const d = (action: Decision["action"], over: Partial<Decision> = {}): Decision => ({
  id: ++n,
  action,
  actor_id: THEM,
  is_power: false,
  created_at: "2026-10-08T10:00:00Z",
  ...over,
});

describe("timelineEntries", () => {
  test("names who acted, and says 'you' for the viewer", () => {
    const [a, b] = timelineEntries([d("approve"), d("dispute", { actor_id: ME })], { me: ME, names });
    expect(a).toMatchObject({ key: "Timeline.approve", who: "other", name: "Andi" });
    expect(b).toMatchObject({ key: "Timeline.dispute", who: "self", name: "Bima" });
  });

  test("a decision with no actor is the system's", () => {
    const [a] = timelineEntries([d("auto_approve", { actor_id: null })], { me: ME, names });
    expect(a).toMatchObject({ who: "system", tone: "system" });
  });

  test("power actions are marked as power, human decisions as human, the rest are plain", () => {
    const tones = timelineEntries(
      [d("override", { is_power: true }), d("uphold", { is_power: true }), d("dismiss", { is_power: true }), d("reject"), d("approve"), d("dispute"), d("submit"), d("rest")],
      { me: ME, names },
    ).map((e) => e.tone);
    expect(tones).toEqual(["power", "power", "power", "human", "human", "human", "plain", "plain"]);
  });

  test("keeps the reason and the order it was given", () => {
    const rows = timelineEntries([d("reject", { reason: "Belum lengkap" }), d("dispute", { reason: "Saya sudah kirim" })], { me: ME, names });
    expect(rows.map((r) => r.reason)).toEqual(["Belum lengkap", "Saya sudah kirim"]);
  });

  test("a power flag wins over the system actor (a defensive case: it never happens, but it must never be hidden)", () => {
    const [a] = timelineEntries([d("override", { actor_id: null, is_power: true })], { me: ME, names });
    expect(a?.tone).toBe("power");
  });
});

describe("overrideDecision", () => {
  test("finds the override that made a rejection final, with its reason", () => {
    const found = overrideDecision([d("submit"), d("auto_approve", { actor_id: null }), d("override", { is_power: true, reason: "Fotonya bukan soal hari ini" })]);
    expect(found?.reason).toBe("Fotonya bukan soal hari ini");
  });

  test("is undefined when nobody overrode", () => {
    expect(overrideDecision([d("submit"), d("reject")])).toBeUndefined();
  });
});

describe("nextDeadline", () => {
  const ci = (status: CheckIn["status"], over: Partial<CheckIn> = {}): CheckIn => ({
    id: "c",
    pact_id: "p",
    member_id: ME,
    reviewer_id: THEM,
    local_date: "2026-10-08",
    status,
    is_final: false,
    cutoff_at: "2026-10-08T16:59:00Z",
    submit_deadline: "2026-10-08T17:29:00Z",
    penalty_applied: false,
    ...over,
  });

  test("each status counts to the deadline that matters for it", () => {
    expect(nextDeadline(ci("open"))).toEqual({ kind: "submit", at: "2026-10-08T17:29:00Z" });
    expect(nextDeadline(ci("submitted", { review_deadline: "2026-10-09T16:59:00Z" }))).toEqual({ kind: "review", at: "2026-10-09T16:59:00Z" });
    expect(nextDeadline(ci("auto_approved", { override_deadline: "2026-10-11T00:00:00Z" }))).toEqual({ kind: "override", at: "2026-10-11T00:00:00Z" });
    expect(nextDeadline(ci("rejected", { dispute_deadline: "2026-10-10T00:00:00Z" }))).toEqual({ kind: "dispute", at: "2026-10-10T00:00:00Z" });
    expect(nextDeadline(ci("disputed", { resolution_deadline: "2026-10-12T00:00:00Z" }))).toEqual({ kind: "resolution", at: "2026-10-12T00:00:00Z" });
  });

  test("a final check-in has none, and neither does one whose deadline is missing", () => {
    expect(nextDeadline(ci("approved", { is_final: true }))).toBeNull();
    expect(nextDeadline(ci("rejected", { is_final: true, dispute_deadline: "2026-10-10T00:00:00Z" }))).toBeNull();
    expect(nextDeadline(ci("submitted"))).toBeNull();
  });
});
