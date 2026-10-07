import { ApiError } from "./errors";

type Result<T> = { data?: T; error?: unknown; response: Response };

// openapi-fetch resolves with {data, error, response} and has already read the body, so build the
// error from `error` rather than re-reading `response`. TanStack Query wants a throw.
export async function unwrap<T>(call: Promise<Result<T>>): Promise<T> {
  let result: Result<T>;
  try {
    result = await call;
  } catch {
    throw ApiError.network();
  }
  if (result.error !== undefined || !result.response.ok) {
    throw ApiError.fromBody(result.response.status, result.error, result.response.headers);
  }
  return result.data as T;
}
