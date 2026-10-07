import { ApiError, errorMessageKey } from "../api/errors";

// Client-side checks mirror api/openapi.yaml (RegisterRequest, LoginRequest) so a typo is caught
// without a round trip. The server stays the authority: it re-validates and its answer is mapped
// back onto the same fields by `formErrorFromApi`.

export type FieldName = "name" | "email" | "password";
export type FieldErrors = Partial<Record<FieldName, string>>;

// A shape check, not a deliverability check: something@domain.tld with no spaces.
const emailShape = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

function checkEmail(raw: string, errors: FieldErrors): string {
  const email = raw.trim();
  if (!email) errors.email = "Auth.emailRequired";
  else if (!emailShape.test(email) || email.length > 254) errors.email = "Auth.emailInvalid";
  return email;
}

export type Result<T> = { ok: true; value: T } | { ok: false; errors: FieldErrors };

export function validateLogin(input: { email: string; password: string }): Result<{ email: string; password: string }> {
  const errors: FieldErrors = {};
  const email = checkEmail(input.email, errors);
  if (!input.password) errors.password = "Auth.passwordRequired";
  if (Object.keys(errors).length) return { ok: false, errors };
  return { ok: true, value: { email, password: input.password } };
}

export function validateRegister(input: {
  name: string;
  email: string;
  password: string;
}): Result<{ display_name: string; email: string; password: string }> {
  const errors: FieldErrors = {};
  const name = input.name.trim();
  if (!name) errors.name = "Auth.nameRequired";
  else if ([...name].length > 80) errors.name = "Auth.nameLong";
  const email = checkEmail(input.email, errors);
  if ([...input.password].length < 8) errors.password = "Auth.passwordShort";
  else if ([...input.password].length > 128) errors.password = "Auth.passwordLong";
  if (Object.keys(errors).length) return { ok: false, errors };
  return { ok: true, value: { display_name: name, email, password: input.password } };
}

export type FormError = {
  fields: FieldErrors;
  /** Shown above the submit button when no single field is to blame. */
  form?: string;
  /** Values for the message at `form`. */
  params?: { seconds: number };
};

// The server names fields as in the contract (`display_name`); the form calls the same input `name`.
const serverField: Record<string, FieldName> = { display_name: "name", email: "email", password: "password" };

const codeField: Record<string, FieldName> = {
  "auth.invalid_email": "email",
  "auth.email_taken": "email",
  "auth.invalid_name": "name",
  "auth.weak_password": "password",
};

/** Turn what the API (or the network) said into messages for the form's fields. */
export function formErrorFromApi(err: unknown): FormError {
  if (!(err instanceof ApiError)) return { fields: {}, form: "Errors.unknown" };

  const field = codeField[err.code];
  if (field) return { fields: { [field]: `Errors.${errorMessageKey(err.code)}` } };

  if (err.code === "validation.failed") {
    const fields: FieldErrors = {};
    for (const e of err.fieldErrors) {
      const f = serverField[e.field];
      if (f) fields[f] = "Errors.validation_failed";
    }
    return Object.keys(fields).length ? { fields } : { fields: {}, form: "Errors.validation_failed" };
  }

  // Both a wrong email and a wrong password give invalid_credentials, so it is never tied to a field.
  if (err.code === "auth.rate_limited" && err.retryAfterSeconds) {
    return { fields: {}, form: "Errors.auth_rate_limited_wait", params: { seconds: Math.ceil(err.retryAfterSeconds) } };
  }
  return { fields: {}, form: `Errors.${errorMessageKey(err.code)}` };
}
