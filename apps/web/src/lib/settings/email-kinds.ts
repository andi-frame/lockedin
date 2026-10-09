import type { components } from "@/lib/api/schema";

export type EmailKind = components["schemas"]["EmailKind"];

// Display order, and the message key under `Settings.kind` for each. `Record<EmailKind, ...>` makes
// the compiler refuse a kind the contract has but this screen forgot (or the other way round).
const messageKey: Record<EmailKind, string> = {
  proof_submitted: "proofSubmitted",
  proof_rejected: "proofRejected",
  proof_overridden: "proofOverridden",
  proof_auto_approved: "proofAutoApproved",
  terms_changed: "termsChanged",
  terms_signed: "termsSigned",
};

export const EMAIL_KINDS = Object.keys(messageKey) as EmailKind[];
export const emailKindMessage = (kind: EmailKind): string => `kind.${messageKey[kind]}`;
