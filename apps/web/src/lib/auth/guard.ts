// Pure rules for the auth guard in src/proxy.ts. The proxy only sees whether a `tepati_session`
// cookie exists, not whether the server still honours it, so it is a fast first gate and not the
// authority: the (app) layout asks the API who the visitor is (ARCHITECTURE §8).
export const SESSION_COOKIE = "tepati_session";
export const HOME = "/today";

// Pages a visitor may open without a session. `/dev` is the dev-only kitchen sink, which answers
// 404 in production on its own. Invite links are private on purpose: the invitee signs in first and
// then comes back through `next`.
const publicPaths = ["/login", "/register", "/dev"];

export function isPublicPath(pathname: string): boolean {
  return publicPaths.some((p) => pathname === p || pathname.startsWith(`${p}/`));
}

/** A same-site path to return to after login, or `/today`. Anything else could be an open redirect. */
export function safeNext(value: string | string[] | null | undefined): string {
  if (typeof value !== "string" || !value.startsWith("/")) return HOME;
  // "//host" and "/\host" are read as another origin by browsers; control characters can split headers.
  if (value.startsWith("//") || value.startsWith("/\\") || /[\u0000-\u001f\u007f]/.test(value)) return HOME;
  let url: URL;
  try {
    url = new URL(value, "http://tepati.invalid");
  } catch {
    return HOME;
  }
  if (url.origin !== "http://tepati.invalid") return HOME;
  if (isPublicPath(url.pathname)) return HOME;
  return value;
}

export function loginRedirect(pathname: string, search: string): string {
  if (pathname === "/") return "/login";
  const params = new URLSearchParams({ next: pathname + search });
  return `/login?${params.toString()}`;
}

export type GuardInput = { pathname: string; search: string; hasSession: boolean };

/** `{ redirect }` when the request should be sent elsewhere, `null` to let it through. */
export function guard({ pathname, search, hasSession }: GuardInput): { redirect: string } | null {
  // `/` is the landing page for a visitor without a session, and the app for one with a cookie.
  if (pathname === "/") return hasSession ? { redirect: HOME } : null;
  if (isPublicPath(pathname)) return null;
  if (!hasSession) return { redirect: loginRedirect(pathname, search) };
  return null;
}
