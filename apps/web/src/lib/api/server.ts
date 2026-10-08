import "server-only";
import { cookies, headers } from "next/headers";
import { createApiClient } from "./client";
import { CSRF_COOKIE } from "./csrf";
import { forwardedHeaders } from "./forward";

const internalUrl = () => process.env.API_INTERNAL_URL ?? "http://localhost:8080";

// For Server Components and server actions: talks to the Go API directly (not through the
// browser origin) and forwards the visitor's cookies so the session carries over.
export async function serverApi() {
  const [jar, incoming] = await Promise.all([cookies(), headers()]);
  return createApiClient({
    baseUrl: `${internalUrl()}/api/v1`,
    headers: { cookie: jar.toString(), ...forwardedHeaders(incoming) },
    csrfToken: () => jar.get(CSRF_COOKIE)?.value,
  });
}
