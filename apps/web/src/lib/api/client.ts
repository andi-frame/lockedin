import createClient, { type Middleware } from "openapi-fetch";
import { CSRF_HEADER, browserCsrfToken } from "./csrf";
import { IDEMPOTENCY_HEADER, newIdempotencyKey } from "./idempotency";
import type { paths } from "./schema";

const SAFE = new Set(["GET", "HEAD", "OPTIONS"]);

export type ApiClientOptions = {
  baseUrl: string;
  fetch?: typeof globalThis.fetch;
  /** Extra headers on every request, e.g. the forwarded cookie on the server. */
  headers?: Record<string, string>;
  /** Where the CSRF token comes from. Defaults to the readable cookie in the browser. */
  csrfToken?: () => string | undefined;
};

export function createApiClient(opts: ApiClientOptions) {
  const csrf = opts.csrfToken ?? browserCsrfToken;
  const client = createClient<paths>({ baseUrl: opts.baseUrl, fetch: opts.fetch, headers: opts.headers });

  const security: Middleware = {
    onRequest({ request }) {
      if (SAFE.has(request.method)) return request;
      const token = csrf();
      if (token) request.headers.set(CSRF_HEADER, token);
      // Callers that retry one action pass their own key; this is the safety net for one-shot calls.
      if (!request.headers.has(IDEMPOTENCY_HEADER)) request.headers.set(IDEMPOTENCY_HEADER, newIdempotencyKey());
      return request;
    },
  };
  client.use(security);
  return client;
}

export type ApiClient = ReturnType<typeof createApiClient>;
