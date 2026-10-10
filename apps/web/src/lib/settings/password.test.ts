import { describe, expect, test } from "bun:test";
import { validatePasswordChange } from "./password";

describe("validatePasswordChange", () => {
  test("a current and a long enough new password pass", () => {
    expect(validatePasswordChange({ current: "belajar-terus", next: "kata-sandi-baru" })).toEqual({ ok: true });
  });

  test("both fields are required, and each error names its field", () => {
    expect(validatePasswordChange({ current: "", next: "" })).toEqual({ ok: false, errors: { current: "Auth.passwordRequired", next: "Auth.passwordShort" } });
  });

  test("the new password follows the server's length rule, counted in characters not bytes", () => {
    expect(validatePasswordChange({ current: "x", next: "1234567" })).toEqual({ ok: false, errors: { next: "Auth.passwordShort" } });
    expect(validatePasswordChange({ current: "x", next: "12345678" })).toEqual({ ok: true });
    // Eight emoji are eight characters although they are many bytes.
    expect(validatePasswordChange({ current: "x", next: "😀".repeat(8) })).toEqual({ ok: true });
    expect(validatePasswordChange({ current: "x", next: "a".repeat(129) })).toEqual({ ok: false, errors: { next: "Auth.passwordLong" } });
    expect(validatePasswordChange({ current: "x", next: "a".repeat(128) })).toEqual({ ok: true });
  });
});
