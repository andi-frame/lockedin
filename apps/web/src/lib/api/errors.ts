import idMessages from "../../../messages/id.json";

export type FieldError = { field: string; message: string };

type ProblemBody = {
  title?: unknown;
  status?: unknown;
  code?: unknown;
  detail?: unknown;
  request_id?: unknown;
  errors?: unknown;
};

const str = (v: unknown): string | undefined => (typeof v === "string" ? v : undefined);

function fieldErrors(v: unknown): FieldError[] {
  if (!Array.isArray(v)) return [];
  const out: FieldError[] = [];
  for (const e of v) {
    if (e && typeof e === "object" && "field" in e && "message" in e) {
      const { field, message } = e as { field: unknown; message: unknown };
      if (typeof field === "string" && typeof message === "string") out.push({ field, message });
    }
  }
  return out;
}

// Branch on `code`, never on `title` or `detail` (api/openapi.yaml, Conventions).
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    readonly requestId?: string,
    readonly fieldErrors: FieldError[] = [],
    readonly retryAfterSeconds?: number,
    title?: string,
  ) {
    super(title ?? code);
    this.name = "ApiError";
  }

  static fromBody(status: number, body: unknown, headers?: Headers): ApiError {
    const b: ProblemBody = body && typeof body === "object" ? (body as ProblemBody) : {};
    const retry = Number(headers?.get("retry-after"));
    const retryAfter = Number.isFinite(retry) && retry > 0 ? retry : undefined;
    const code = str(b.code) ?? (status >= 500 ? "server.unavailable" : "unknown");
    return new ApiError(status, code, str(b.request_id), fieldErrors(b.errors), retryAfter, str(b.title));
  }

  static async fromResponse(res: Response): Promise<ApiError> {
    let body: unknown;
    try {
      body = await res.json();
    } catch {
      // A proxy error page or an empty body: fall through to the status-based code.
    }
    return ApiError.fromBody(res.status, body, res.headers);
  }

  /** fetch itself failed (offline, DNS, connection refused): there is no HTTP status. */
  static network(): ApiError {
    return new ApiError(0, "network");
  }
}

// next-intl reads "." in a message key as nesting, so API codes like "auth.email_taken" are stored
// as "auth_email_taken" in messages/*.json.
const toKey = (code: string) => code.replaceAll(".", "_");
const known = new Set(Object.keys(idMessages.Errors));

// Key into the `Errors` namespace of messages/*.json. Codes the web has no copy for yet degrade to
// a generic line instead of leaking the English title.
export function errorMessageKey(code: string): string {
  return known.has(toKey(code)) ? toKey(code) : "unknown";
}
