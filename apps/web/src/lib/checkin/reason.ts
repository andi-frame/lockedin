// A reject, an override, a dispute and a dismissal each need a written reason: at least 10
// characters after trimming, counted in characters and not bytes (the contract's ReasonRequest and
// the DB check on `decisions`), and at most 1000.
export const REASON_MIN = 10;
export const REASON_MAX = 1000;

export type ReasonState = {
  ok: boolean;
  /** What is sent: the text without the spaces around it. */
  trimmed: string;
  length: number;
  /** Characters still needed to reach the minimum. */
  missing: number;
  tooLong: boolean;
};

export function reasonState(raw: string): ReasonState {
  const trimmed = raw.trim();
  // Array.from walks code points, so an emoji is one character like the server counts it.
  const length = Array.from(trimmed).length;
  const tooLong = length > REASON_MAX;
  return { ok: length >= REASON_MIN && !tooLong, trimmed, length, missing: Math.max(0, REASON_MIN - length), tooLong };
}
