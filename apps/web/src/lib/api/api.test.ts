import { describe, expect, test } from "bun:test";
import { createApiClient } from "./client";
import { readCookie } from "./csrf";
import { ApiError, errorMessageKey } from "./errors";
import { unwrap } from "./unwrap";
import { newIdempotencyKey } from "./idempotency";

type Seen = { url: string; method: string; headers: Headers };

// A fetch that records the request it was given and answers with `reply`.
function recorder(reply: () => Response) {
  const seen: Seen[] = [];
  const fetch = async (input: Request) => {
    seen.push({ url: input.url, method: input.method, headers: new Headers(input.headers) });
    return reply();
  };
  return { seen, fetch: fetch as unknown as typeof globalThis.fetch };
}

const json = (body: unknown, status = 200, type = "application/json") =>
  new Response(JSON.stringify(body), { status, headers: { "content-type": type } });

describe("readCookie", () => {
  test("finds a cookie among several", () => {
    expect(readCookie("tepati_csrf", "a=1; tepati_csrf=tok.en; b=2")).toBe("tok.en");
  });
  test("decodes percent-encoding", () => {
    expect(readCookie("x", "x=a%20b")).toBe("a b");
  });
  test("does not match a longer name", () => {
    expect(readCookie("csrf", "tepati_csrf=nope")).toBeUndefined();
  });
  test("is undefined for an empty jar", () => {
    expect(readCookie("tepati_csrf", "")).toBeUndefined();
  });
});

describe("newIdempotencyKey", () => {
  test("is unique and long enough for the contract (8-128 chars)", () => {
    const a = newIdempotencyKey();
    const b = newIdempotencyKey();
    expect(a).not.toBe(b);
    expect(a.length).toBeGreaterThanOrEqual(8);
    expect(a.length).toBeLessThanOrEqual(128);
  });
});

describe("createApiClient", () => {
  test("sends the CSRF header and an Idempotency-Key on unsafe methods", async () => {
    const { seen, fetch } = recorder(() => json({}));
    const api = createApiClient({ baseUrl: "http://x/api/v1", fetch, csrfToken: () => "csrf-1" });
    await api.POST("/auth/logout");
    expect(seen[0]?.headers.get("x-csrf-token")).toBe("csrf-1");
    expect(seen[0]?.headers.get("idempotency-key")?.length).toBeGreaterThanOrEqual(8);
  });

  test("keeps a caller-supplied Idempotency-Key so a retry replays", async () => {
    const { seen, fetch } = recorder(() => json({}));
    const api = createApiClient({ baseUrl: "http://x/api/v1", fetch, csrfToken: () => "c" });
    await api.POST("/auth/logout", { headers: { "Idempotency-Key": "fixed-key-1" } });
    expect(seen[0]?.headers.get("idempotency-key")).toBe("fixed-key-1");
  });

  test("adds neither header to a GET", async () => {
    const { seen, fetch } = recorder(() => json({}));
    const api = createApiClient({ baseUrl: "http://x/api/v1", fetch, csrfToken: () => "c" });
    await api.GET("/me");
    expect(seen[0]?.headers.get("x-csrf-token")).toBeNull();
    expect(seen[0]?.headers.get("idempotency-key")).toBeNull();
  });

  test("omits the CSRF header when there is no token yet", async () => {
    const { seen, fetch } = recorder(() => json({}));
    const api = createApiClient({ baseUrl: "http://x/api/v1", fetch, csrfToken: () => undefined });
    await api.POST("/auth/logout");
    expect(seen[0]?.headers.has("x-csrf-token")).toBe(false);
  });

  test("forwards extra headers such as the cookie", async () => {
    const { seen, fetch } = recorder(() => json({}));
    const api = createApiClient({ baseUrl: "http://x/api/v1", fetch, headers: { cookie: "tepati_session=s" } });
    await api.GET("/me");
    expect(seen[0]?.headers.get("cookie")).toBe("tepati_session=s");
  });
});

describe("ApiError", () => {
  test("reads a problem+json body", async () => {
    const res = json(
      { type: "about:blank", title: "Nope", status: 409, code: "auth.email_taken", request_id: "r1" },
      409,
      "application/problem+json",
    );
    const err = await ApiError.fromResponse(res);
    expect(err.status).toBe(409);
    expect(err.code).toBe("auth.email_taken");
    expect(err.requestId).toBe("r1");
  });

  test("keeps per-field detail", async () => {
    const res = json(
      { title: "bad", status: 400, code: "validation.failed", errors: [{ field: "email", message: "required" }] },
      400,
      "application/problem+json",
    );
    expect((await ApiError.fromResponse(res)).fieldErrors).toEqual([{ field: "email", message: "required" }]);
  });

  test("falls back to the status when the body is not a problem", async () => {
    const err = await ApiError.fromResponse(new Response("<html>bad gateway</html>", { status: 502 }));
    expect(err.status).toBe(502);
    expect(err.code).toBe("server.unavailable");
  });

  test("reads Retry-After in seconds", async () => {
    const res = new Response(JSON.stringify({ title: "busy", status: 503, code: "upload.queue_busy" }), {
      status: 503,
      headers: { "content-type": "application/problem+json", "retry-after": "10" },
    });
    expect((await ApiError.fromResponse(res)).retryAfterSeconds).toBe(10);
  });
});

describe("errorMessageKey", () => {
  // next-intl forbids "." in message keys (it means nesting), so dots become underscores.
  test("maps a known code to its message key, with underscores for dots", () => {
    expect(errorMessageKey("auth.unauthenticated")).toBe("auth_unauthenticated");
    expect(errorMessageKey("rate_limited")).toBe("rate_limited");
  });
  test("falls back to unknown for a code the web does not know", () => {
    expect(errorMessageKey("something.new")).toBe("unknown");
  });
});

describe("unwrap", () => {
  test("returns data on success", async () => {
    const me = {
      id: "u1",
      email: "a@b.test",
      display_name: "A",
      locale: "id" as const,
      timezone: "Asia/Jakarta",
      email_kinds_off: [],
      created_at: "2026-10-08T00:00:00Z",
    };
    const { fetch } = recorder(() => json(me));
    const api = createApiClient({ baseUrl: "http://x/api/v1", fetch });
    expect(await unwrap(api.GET("/me"))).toEqual(me);
  });

  test("throws an ApiError built from the body openapi-fetch already read", async () => {
    const { fetch } = recorder(() =>
      json({ title: "x", status: 409, code: "auth.email_taken", request_id: "r9" }, 409, "application/problem+json"),
    );
    const api = createApiClient({ baseUrl: "http://x/api/v1", fetch });
    const err = await unwrap(api.GET("/me")).catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).code).toBe("auth.email_taken");
    expect((err as ApiError).requestId).toBe("r9");
  });

  test("returns undefined for 204", async () => {
    const { fetch } = recorder(() => new Response(null, { status: 204 }));
    const api = createApiClient({ baseUrl: "http://x/api/v1", fetch });
    expect(await unwrap(api.POST("/auth/logout"))).toBeUndefined();
  });

  test("turns a network failure into a network ApiError", async () => {
    const fetch = (async () => {
      throw new TypeError("fetch failed");
    }) as unknown as typeof globalThis.fetch;
    const api = createApiClient({ baseUrl: "http://x/api/v1", fetch });
    const err = await unwrap(api.GET("/me")).catch((e: unknown) => e);
    expect((err as ApiError).code).toBe("network");
    expect((err as ApiError).status).toBe(0);
  });
});
