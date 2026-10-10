// Mirrors api/openapi.yaml (ChangePasswordRequest) so a typo is caught without a round trip; the
// server stays the authority. Lengths count characters, as the server does.
export type PasswordFieldErrors = Partial<Record<"current" | "next", string>>;
export type PasswordCheck = { ok: true } | { ok: false; errors: PasswordFieldErrors };

export function validatePasswordChange(input: { current: string; next: string }): PasswordCheck {
  const errors: PasswordFieldErrors = {};
  if (!input.current) errors.current = "Auth.passwordRequired";
  const n = [...input.next].length;
  if (n < 8) errors.next = "Auth.passwordShort";
  else if (n > 128) errors.next = "Auth.passwordLong";
  return Object.keys(errors).length ? { ok: false, errors } : { ok: true };
}
