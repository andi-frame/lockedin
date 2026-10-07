// One key per logical user action (a click on "Kirim"), not per HTTP attempt: a retry of the same
// action must reuse it so the server replays the stored response instead of acting twice.
export const IDEMPOTENCY_HEADER = "Idempotency-Key";

export function newIdempotencyKey(): string {
  return crypto.randomUUID();
}
