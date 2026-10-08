import { describe, expect, test } from "bun:test";
import type { components } from "../api/schema";
import { agreementState, checkSignature, inviteUrl } from "./agreement";

type Pact = components["schemas"]["Pact"];
type Member = components["schemas"]["PactMember"];

describe("checkSignature", () => {
  const ok = { agreed: true, typed: "Sari Dewi", displayName: "Sari Dewi" };

  test("accepts the exact name once the box is ticked", () => expect(checkSignature(ok)).toEqual({ ok: true }));

  test("ignores case and surrounding spaces, as the server does", () => {
    expect(checkSignature({ ...ok, typed: "  sari dewi " })).toEqual({ ok: true });
  });

  test("the box must be ticked", () => {
    expect(checkSignature({ ...ok, agreed: false })).toEqual({ ok: false, errors: { agreed: "Sign.agreeRequired" } });
  });

  test("a blank name asks for the name, a wrong one says it does not match", () => {
    expect(checkSignature({ ...ok, typed: "  " })).toEqual({ ok: false, errors: { name: "Sign.nameRequired" } });
    expect(checkSignature({ ...ok, typed: "Sari" })).toEqual({ ok: false, errors: { name: "Sign.nameMismatch" } });
  });

  test("reports both problems together", () => {
    expect(checkSignature({ agreed: false, typed: "x", displayName: "Sari Dewi" })).toEqual({
      ok: false,
      errors: { agreed: "Sign.agreeRequired", name: "Sign.nameMismatch" },
    });
  });

  test("a composed and a decomposed é are the same name", () => {
    expect(checkSignature({ agreed: true, typed: "René", displayName: "René" })).toEqual({ ok: true });
  });
});

describe("inviteUrl", () => {
  test("is the origin plus /invite/<token>", () => {
    expect(inviteUrl("https://tepati.test", "abc_DEF-123")).toBe("https://tepati.test/invite/abc_DEF-123");
  });
  test("a trailing slash on the origin does not double up", () => {
    expect(inviteUrl("https://tepati.test/", "t")).toBe("https://tepati.test/invite/t");
  });
  test("the token is escaped", () => expect(inviteUrl("https://x.test", "a/b c")).toBe("https://x.test/invite/a%2Fb%20c"));
});

describe("agreementState", () => {
  const member = (user_id: string, role: "backer" | "doer", accepted: boolean): Member => ({
    user_id,
    display_name: user_id,
    role,
    line_color: "#0F766E",
    accepted,
    accepted_at: accepted ? "2026-10-08T00:00:00Z" : null,
    signature_name: accepted ? user_id : null,
    rest_days_used: 0,
  });
  const pact = (over: Partial<Pact>): Pact =>
    ({ status: "proposed", terms_version: 1, members: [member("b", "backer", false)], ...over }) as Pact;

  test("waiting for the invitee while only the backer is in", () => {
    expect(agreementState(pact({}), "b")).toEqual({
      phase: "waiting_for_doer",
      iSigned: false,
      canSign: false,
      termsChanged: false,
    });
  });

  test("both present, nobody signed: each can sign", () => {
    const p = pact({ members: [member("b", "backer", false), member("d", "doer", false)] });
    expect(agreementState(p, "b")).toMatchObject({ phase: "signing", iSigned: false, canSign: true });
    expect(agreementState(p, "d")).toMatchObject({ phase: "signing", iSigned: false, canSign: true });
  });

  test("a member who signed cannot sign again, the other still can", () => {
    const p = pact({ members: [member("b", "backer", true), member("d", "doer", false)] });
    expect(agreementState(p, "b")).toMatchObject({ iSigned: true, canSign: false });
    expect(agreementState(p, "d")).toMatchObject({ iSigned: false, canSign: true });
  });

  test("a second version means the signatures were cleared by an edit", () => {
    const p = pact({ terms_version: 2, members: [member("b", "backer", false), member("d", "doer", false)] });
    expect(agreementState(p, "b").termsChanged).toBe(true);
  });

  test("once someone has signed the current version, the note about clearing goes away", () => {
    const p = pact({ terms_version: 2, members: [member("b", "backer", true), member("d", "doer", false)] });
    expect(agreementState(p, "d").termsChanged).toBe(false);
  });

  test("a draft has no agreement yet, and a scheduled pact is past it", () => {
    expect(agreementState(pact({ status: "draft" }), "b").phase).toBe("draft");
    expect(agreementState(pact({ status: "scheduled" }), "b")).toMatchObject({ phase: "done", canSign: false });
    expect(agreementState(pact({ status: "cancelled" }), "b").phase).toBe("done");
  });
});
