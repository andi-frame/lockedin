// The Go API sets a readable `tepati_csrf` cookie at login; unsafe requests copy it into the
// X-CSRF-Token header (double-submit, ARCHITECTURE §8).
export const CSRF_COOKIE = "tepati_csrf";
export const CSRF_HEADER = "X-CSRF-Token";

export function readCookie(name: string, jar: string): string | undefined {
  for (const part of jar.split(";")) {
    const eq = part.indexOf("=");
    if (eq < 0) continue;
    if (part.slice(0, eq).trim() !== name) continue;
    const raw = part.slice(eq + 1).trim();
    try {
      return decodeURIComponent(raw);
    } catch {
      return raw;
    }
  }
  return undefined;
}

export function browserCsrfToken(): string | undefined {
  if (typeof document === "undefined") return undefined;
  return readCookie(CSRF_COOKIE, document.cookie);
}
