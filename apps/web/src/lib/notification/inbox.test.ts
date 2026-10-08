import { describe, expect, test } from "bun:test";
import en from "../../../messages/en.json";
import id from "../../../messages/id.json";
import type { components } from "../api/schema";
import { checkInIdsToResolve, KIND_TARGET, notificationHref, payloadRefs, unreadIds } from "./inbox";

type Notification = components["schemas"]["Notification"];

const PACT = "11111111-1111-4111-8111-111111111111";
const CHECK_IN = "22222222-2222-4222-8222-222222222222";
const OWNER = "33333333-3333-4333-8333-333333333333";

function note(kind: Notification["kind"], payload: Record<string, unknown>, extra: Partial<Notification> = {}): Notification {
  return { id: 1, kind, payload, created_at: "2026-10-08T01:00:00Z", read_at: null, ...extra };
}

describe("payloadRefs", () => {
  test("reads the pact and check-in ids", () => {
    expect(payloadRefs(note("proof_approved", { pact_id: PACT, check_in_id: CHECK_IN }))).toEqual({ pactId: PACT, checkInId: CHECK_IN });
  });

  test("a missing or malformed id is null, so a bad payload never builds a URL", () => {
    expect(payloadRefs(note("pact_settled", { pact_id: PACT }))).toEqual({ pactId: PACT, checkInId: null });
    expect(payloadRefs(note("pact_settled", { pact_id: "../../etc", check_in_id: 7 }))).toEqual({ pactId: null, checkInId: null });
    expect(payloadRefs(note("pact_settled", {}))).toEqual({ pactId: null, checkInId: null });
  });
});

describe("notificationHref", () => {
  const resolved = { local_date: "2026-10-07", member_id: OWNER };

  test("a check-in kind opens that day, for the member who owes it", () => {
    const n = note("proof_submitted", { pact_id: PACT, check_in_id: CHECK_IN });
    expect(notificationHref(n, resolved)).toBe(`/pacts/${PACT}/days/2026-10-07?of=${OWNER}`);
  });

  test("a check-in kind whose check-in could not be read falls back to the pact", () => {
    const n = note("proof_submitted", { pact_id: PACT, check_in_id: CHECK_IN });
    expect(notificationHref(n, null)).toBe(`/pacts/${PACT}`);
    expect(notificationHref(n, undefined)).toBe(`/pacts/${PACT}`);
  });

  test("a check-in kind without a check-in id falls back to the pact", () => {
    expect(notificationHref(note("proof_rejected", { pact_id: PACT }), resolved)).toBe(`/pacts/${PACT}`);
  });

  test("a pact kind opens the pact even when a check-in is known", () => {
    expect(notificationHref(note("pact_settled", { pact_id: PACT, check_in_id: CHECK_IN }), resolved)).toBe(`/pacts/${PACT}`);
  });

  test("no pact id, no link", () => {
    expect(notificationHref(note("pact_settled", {}), resolved)).toBeNull();
  });
});

describe("checkInIdsToResolve", () => {
  test("lists each check-in once, only for kinds that open a check-in", () => {
    const items = [
      note("proof_submitted", { pact_id: PACT, check_in_id: CHECK_IN }, { id: 1 }),
      note("proof_edited", { pact_id: PACT, check_in_id: CHECK_IN }, { id: 2 }),
      note("pact_settled", { pact_id: PACT, check_in_id: "44444444-4444-4444-8444-444444444444" }, { id: 3 }),
      note("day_missed", { pact_id: PACT }, { id: 4 }),
    ];
    expect(checkInIdsToResolve(items)).toEqual([CHECK_IN]);
  });
});

describe("unreadIds", () => {
  test("only the ones not yet read", () => {
    const items = [note("proof_approved", {}, { id: 5 }), note("proof_approved", {}, { id: 6, read_at: "2026-10-08T02:00:00Z" }), note("day_missed", {}, { id: 7 })];
    expect(unreadIds(items)).toEqual([5, 7]);
  });
});

describe("copy for every kind", () => {
  const kinds = Object.keys(KIND_TARGET);

  test("covers all 22 kinds of the API", () => {
    expect(kinds).toHaveLength(22);
  });

  test.each(kinds)("%s has a sentence in Indonesian and English", (kind) => {
    const idCopy = id.Notifications.kinds as Record<string, string>;
    const enCopy = en.Notifications.kinds as Record<string, string>;
    expect(idCopy[kind]?.trim().length).toBeGreaterThan(0);
    expect(enCopy[kind]?.trim().length).toBeGreaterThan(0);
  });
});
