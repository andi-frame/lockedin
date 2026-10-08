import { NextResponse, type NextRequest } from "next/server";
import { guard, SESSION_COOKIE } from "@/lib/auth/guard";

// A fast first gate: no session cookie, no private page. It cannot tell a live session from a
// stale cookie, so the (app) layout still asks the API (see lib/auth/guard.ts).
export function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl;
  const decision = guard({ pathname, search, hasSession: request.cookies.has(SESSION_COOKIE) });
  if (decision) return NextResponse.redirect(new URL(decision.redirect, request.url));
  return NextResponse.next();
}

export const config = {
  // Not the API (proxied to Go), Next internals, or files with an extension (icons, fonts, images).
  matcher: ["/((?!api|_next/static|_next/image|favicon.ico|.*\\..*).*)"],
};
