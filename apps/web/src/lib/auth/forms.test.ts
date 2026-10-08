import { describe, expect, test } from "bun:test";
import enMessages from "../../../messages/en.json";
import idMessages from "../../../messages/id.json";
import { ApiError } from "../api/errors";
import { errorMessageKey } from "../api/errors";
import { formErrorFromApi, validateLogin, validateRegister } from "./forms";

const lookup = (messages: unknown, path: string): unknown =>
  path.split(".").reduce<unknown>((node, part) => (node && typeof node === "object" ? (node as Record<string, unknown>)[part] : undefined), messages);

describe("validateLogin", () => {
  test("accepts an email and a password and trims the email", () => {
    expect(validateLogin({ email: "  sari@kampus.ac.id ", password: "x" })).toEqual({
      ok: true,
      value: { email: "sari@kampus.ac.id", password: "x" },
    });
  });
  test("asks for both fields when they are empty", () => {
    expect(validateLogin({ email: "", password: "" })).toEqual({
      ok: false,
      errors: { email: "Auth.emailRequired", password: "Auth.passwordRequired" },
    });
  });
  test("does not judge the length of an existing password", () => {
    // An older, shorter password must still be able to sign in; only the server decides.
    expect(validateLogin({ email: "a@b.id", password: "abc" }).ok).toBe(true);
  });
  test.each(["sari", "sari@", "@kampus.id", "sari kampus@x.id", "sari@kampus"])("rejects the email %p", (email) => {
    expect(validateLogin({ email, password: "x" })).toEqual({ ok: false, errors: { email: "Auth.emailInvalid" } });
  });
});

describe("validateRegister", () => {
  const good = { name: "Sari Wulandari", email: "sari@kampus.ac.id", password: "kata-sandi-1" };

  test("accepts a complete form and trims name and email", () => {
    expect(validateRegister({ ...good, name: "  Sari ", email: " sari@kampus.ac.id " })).toEqual({
      ok: true,
      value: { display_name: "Sari", email: "sari@kampus.ac.id", password: "kata-sandi-1" },
    });
  });
  test("leaves the password exactly as typed", () => {
    const r = validateRegister({ ...good, password: "  spasi di tepi  " });
    expect(r.ok && r.value.password).toBe("  spasi di tepi  ");
  });
  test("requires a name, and counts only what is left after trimming", () => {
    expect(validateRegister({ ...good, name: "   " })).toEqual({ ok: false, errors: { name: "Auth.nameRequired" } });
  });
  test("a name over 80 characters is too long", () => {
    expect(validateRegister({ ...good, name: "a".repeat(81) })).toEqual({ ok: false, errors: { name: "Auth.nameLong" } });
    expect(validateRegister({ ...good, name: "a".repeat(80) }).ok).toBe(true);
  });
  test("a password under 8 characters is too short, 8 is enough", () => {
    expect(validateRegister({ ...good, password: "1234567" })).toEqual({ ok: false, errors: { password: "Auth.passwordShort" } });
    expect(validateRegister({ ...good, password: "12345678" }).ok).toBe(true);
  });
  test("a password over 128 characters is too long", () => {
    expect(validateRegister({ ...good, password: "a".repeat(129) })).toEqual({ ok: false, errors: { password: "Auth.passwordLong" } });
  });
  test("reports every bad field at once so the form is fixed in one pass", () => {
    const r = validateRegister({ name: "", email: "x", password: "1" });
    expect(r).toEqual({
      ok: false,
      errors: { name: "Auth.nameRequired", email: "Auth.emailInvalid", password: "Auth.passwordShort" },
    });
  });
});

describe("formErrorFromApi", () => {
  const problem = (code: string, status = 400) => ApiError.fromBody(status, { code, title: "t" });

  test("a taken email is shown on the email field", () => {
    expect(formErrorFromApi(problem("auth.email_taken", 409))).toEqual({ fields: { email: "Errors.auth_email_taken" } });
  });
  test.each([
    ["auth.invalid_email", "email"],
    ["auth.invalid_name", "name"],
    ["auth.weak_password", "password"],
  ] as const)("%s lands on the %s field", (code, field) => {
    const e = formErrorFromApi(problem(code));
    expect(Object.keys(e.fields)).toEqual([field]);
    expect(e.form).toBeUndefined();
  });
  // Naming which of the two was wrong would let anyone test whether an email has an account.
  test("wrong credentials stay at form level and name neither field", () => {
    expect(formErrorFromApi(problem("auth.invalid_credentials", 401))).toEqual({
      fields: {},
      form: "Errors.auth_invalid_credentials",
    });
  });
  test("validation.failed puts a generic line on each field the server named", () => {
    const e = formErrorFromApi(
      ApiError.fromBody(422, {
        code: "validation.failed",
        errors: [
          { field: "display_name", message: "required" },
          { field: "password", message: "too short" },
        ],
      }),
    );
    expect(e.fields).toEqual({ name: "Errors.validation_failed", password: "Errors.validation_failed" });
  });
  test("validation.failed without a field we know is a form-level error", () => {
    const e = formErrorFromApi(ApiError.fromBody(422, { code: "validation.failed", errors: [{ field: "locale", message: "x" }] }));
    expect(e).toEqual({ fields: {}, form: "Errors.validation_failed" });
  });
  test("a rate limit says how long to wait when the server said so", () => {
    const e = formErrorFromApi(ApiError.fromBody(429, { code: "auth.rate_limited" }, new Headers({ "retry-after": "42" })));
    expect(e).toEqual({ fields: {}, form: "Errors.auth_rate_limited_wait", params: { seconds: 42 } });
  });
  test("a rate limit without Retry-After still has a message", () => {
    const e = formErrorFromApi(problem("auth.rate_limited", 429));
    expect(e.form).toBe("Errors.auth_rate_limited");
    expect(e.params).toBeUndefined();
  });
  test("no connection and server failures are form-level", () => {
    expect(formErrorFromApi(ApiError.network()).form).toBe("Errors.network");
    expect(formErrorFromApi(problem("server.internal", 500)).form).toBe("Errors.server_internal");
  });
  test("a code the web has no copy for degrades to the generic line", () => {
    expect(formErrorFromApi(problem("something.new")).form).toBe("Errors.unknown");
  });
  test("anything that is not an ApiError is the generic line", () => {
    expect(formErrorFromApi(new TypeError("boom")).form).toBe("Errors.unknown");
  });
});

describe("copy for the auth forms", () => {
  // Every code these two forms can receive, from the x-problem-codes lines in api/openapi.yaml.
  const codes = [
    "validation.failed",
    "auth.invalid_email",
    "auth.invalid_name",
    "auth.weak_password",
    "auth.email_taken",
    "auth.rate_limited",
    "auth.invalid_credentials",
  ];
  test.each(codes)("%s has its own message and is not the generic fallback", (code) => {
    expect(errorMessageKey(code)).not.toBe("unknown");
  });

  const keys = [
    "Auth.emailRequired",
    "Auth.emailInvalid",
    "Auth.passwordRequired",
    "Auth.passwordShort",
    "Auth.passwordLong",
    "Auth.nameRequired",
    "Auth.nameLong",
    ...codes.map((c) => `Errors.${errorMessageKey(c)}`),
    "Errors.auth_rate_limited_wait",
  ];
  test.each(keys)("%s exists in Indonesian and English", (path) => {
    expect(typeof lookup(idMessages, path)).toBe("string");
    expect(typeof lookup(enMessages, path)).toBe("string");
  });
});
