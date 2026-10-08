import type { components } from "../api/schema";

type Decision = components["schemas"]["Decision"];
type CheckIn = components["schemas"]["CheckIn"];

export type TimelineEntry = {
  id: number;
  key: `Timeline.${Decision["action"]}`;
  /** Who acted, from the viewer's side. */
  who: "self" | "other" | "system";
  name: string;
  /**
   * `power` is the backer's extra reach (override, uphold, dismiss): drawn in stamp violet and
   * labelled, so it is never silent (SPEC §2). `human` is any other decision a person takes
   * about a proof. `system` is the worker. The rest is plain.
   */
  tone: "power" | "human" | "system" | "plain";
  reason: string | null;
  at: string;
};

const human = new Set<Decision["action"]>(["approve", "reject", "dispute"]);

export function timelineEntries(decisions: Decision[], ctx: { me: string; names: Record<string, string> }): TimelineEntry[] {
  return decisions.map((d) => {
    const who = d.actor_id === null || d.actor_id === undefined ? "system" : d.actor_id === ctx.me ? "self" : "other";
    return {
      id: d.id,
      key: `Timeline.${d.action}`,
      who,
      name: d.actor_id ? (ctx.names[d.actor_id] ?? "") : "",
      // A power flag is checked first, so it is never shown as the system's.
      tone: d.is_power ? "power" : who === "system" ? "system" : human.has(d.action) ? "human" : "plain",
      reason: d.reason ?? null,
      at: d.created_at,
    };
  });
}

/** The override that made a rejection final, if there is one: the doer is shown it with its reason. */
export function overrideDecision(decisions: Decision[]): Decision | undefined {
  return decisions.find((d) => d.action === "override");
}

export type NextDeadline = { kind: "submit" | "review" | "override" | "dispute" | "resolution"; at: string };

/** The one deadline that matters for where this check-in stands, or null once it is final. */
export function nextDeadline(ci: CheckIn): NextDeadline | null {
  if (ci.is_final) return null;
  const pick = (kind: NextDeadline["kind"], at: string | null | undefined) => (at ? { kind, at } : null);
  switch (ci.status) {
    case "open":
      return pick("submit", ci.submit_deadline);
    case "submitted":
      return pick("review", ci.review_deadline);
    case "auto_approved":
      return pick("override", ci.override_deadline);
    case "rejected":
      return pick("dispute", ci.dispute_deadline);
    case "disputed":
      return pick("resolution", ci.resolution_deadline);
    default:
      return null;
  }
}
