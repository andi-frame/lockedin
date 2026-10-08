import type { components } from "../api/schema";

export type Notification = components["schemas"]["Notification"];
export type NotificationKind = components["schemas"]["NotificationKind"];

/**
 * Where a notification points. A `Record` over the generated enum, so a kind the API adds fails the
 * typecheck here until it is placed (and the copy test fails until it has a sentence).
 * `checkin` kinds open the day of one check-in; `pact` kinds open the pact.
 */
export const KIND_TARGET: Record<NotificationKind, "pact" | "checkin"> = {
  member_joined: "pact",
  terms_changed: "pact",
  terms_signed: "pact",
  pact_scheduled: "pact",
  pact_settled: "pact",
  proof_submitted: "checkin",
  proof_edited: "checkin",
  proof_approved: "checkin",
  proof_rejected: "checkin",
  proof_auto_approved: "checkin",
  proof_overridden: "checkin",
  rejection_final: "checkin",
  rest_declared: "checkin",
  day_missed: "checkin",
  dispute_opened: "checkin",
  dispute_upheld: "checkin",
  dispute_dismissed: "checkin",
  reminder_cutoff_3h: "checkin",
  reminder_cutoff_30m: "checkin",
  review_deadline_soon: "checkin",
  payout_marked_paid: "pact",
  payout_confirmed: "pact",
};

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

function uuidOrNull(v: unknown): string | null {
  return typeof v === "string" && UUID.test(v) ? v : null;
}

/** The ids a notification carries. Anything that is not a UUID is dropped, so it can never end up in a URL. */
export function payloadRefs(n: Pick<Notification, "payload">): { pactId: string | null; checkInId: string | null } {
  return { pactId: uuidOrNull(n.payload.pact_id), checkInId: uuidOrNull(n.payload.check_in_id) };
}

/** What a check-in lookup gives the link: the day, and whose check-in it is (the day page takes `?of=`). */
export type CheckInRef = { local_date: string; member_id: string };

/**
 * The page a notification opens, or null when the payload names no pact. A check-in that could not
 * be read (or was never named) still leads somewhere useful: the pact it belongs to.
 */
export function notificationHref(n: Pick<Notification, "kind" | "payload">, checkIn: CheckInRef | null | undefined): string | null {
  const { pactId, checkInId } = payloadRefs(n);
  if (!pactId) return null;
  if (KIND_TARGET[n.kind] === "checkin" && checkInId && checkIn) return `/pacts/${pactId}/days/${checkIn.local_date}?of=${checkIn.member_id}`;
  return `/pacts/${pactId}`;
}

/** The check-ins to look up before linking: each once, and only for the kinds that open one. */
export function checkInIdsToResolve(items: Pick<Notification, "kind" | "payload">[]): string[] {
  const ids = new Set<string>();
  for (const n of items) {
    if (KIND_TARGET[n.kind] !== "checkin") continue;
    const { checkInId } = payloadRefs(n);
    if (checkInId) ids.add(checkInId);
  }
  return [...ids];
}

export function unreadIds(items: Pick<Notification, "id" | "read_at">[]): number[] {
  return items.filter((n) => !n.read_at).map((n) => n.id);
}
