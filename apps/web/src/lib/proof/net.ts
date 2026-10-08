// Small pure helpers for the upload path.

/** Headers for the presigned PUT, unchanged except Content-Length, which a browser sets by itself. */
export function putHeaders(headers: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(headers).filter(([k]) => k.toLowerCase() !== "content-length"));
}

/** Delay before the next status poll: one second, backing off by half each time up to five. */
export function pollDelay(attempt: number): number {
  return Math.min(5000, Math.round(1000 * 1.5 ** attempt));
}

/** The size to draw an image at so its longest side is at most `max`; never enlarges. */
export function targetSize(width: number, height: number, max: number): { width: number; height: number } {
  const longest = Math.max(width, height);
  if (longest <= max) return { width, height };
  const scale = max / longest;
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
}
