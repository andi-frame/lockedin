import type { components } from "../api/schema";

type Pact = components["schemas"]["Pact"];

export type SignatureCheck = { ok: true } | { ok: false; errors: { agreed?: string; name?: string } };

const fold = (s: string) => s.trim().normalize("NFC").toLowerCase();

/**
 * SPEC §4: signing means ticking the summary and typing your display name. The server compares the
 * name trimmed and case-insensitively (`pact.signature_mismatch`); checking the same way here only
 * saves a round trip.
 */
export function checkSignature(input: { agreed: boolean; typed: string; displayName: string }): SignatureCheck {
  const errors: { agreed?: string; name?: string } = {};
  if (!input.agreed) errors.agreed = "Sign.agreeRequired";
  if (!input.typed.trim()) errors.name = "Sign.nameRequired";
  else if (fold(input.typed) !== fold(input.displayName)) errors.name = "Sign.nameMismatch";
  return Object.keys(errors).length ? { ok: false, errors } : { ok: true };
}

/** The link the backer shares. Only the hash of the token is stored, so it exists in full once. */
export function inviteUrl(origin: string, token: string): string {
  return `${origin.replace(/\/+$/, "")}/invite/${encodeURIComponent(token)}`;
}

export type AgreementState = {
  /** draft: not proposed yet. waiting_for_doer: invited, nobody joined. signing: both in. done: past the agreement. */
  phase: "draft" | "waiting_for_doer" | "signing" | "done";
  iSigned: boolean;
  canSign: boolean;
  /** The terms were edited after version 1 and nobody has signed this version yet (SPEC §3). */
  termsChanged: boolean;
};

export function agreementState(pact: Pact, me: string): AgreementState {
  const mine = pact.members.find((m) => m.user_id === me);
  const iSigned = mine?.accepted ?? false;
  const nobodySigned = pact.members.every((m) => !m.accepted);
  const termsChanged = pact.status === "proposed" && pact.terms_version > 1 && nobodySigned;

  if (pact.status === "draft") return { phase: "draft", iSigned, canSign: false, termsChanged: false };
  if (pact.status !== "proposed") return { phase: "done", iSigned, canSign: false, termsChanged: false };
  if (pact.members.length < 2) return { phase: "waiting_for_doer", iSigned, canSign: false, termsChanged };
  return { phase: "signing", iSigned, canSign: Boolean(mine) && !iSigned, termsChanged };
}
