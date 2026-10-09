// Shared by the server (request.ts) and by client code that sets the cookie. No server-only imports here.
export const locales = ["id", "en"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "id";
export const localeCookie = "tepati_locale";

export function isLocale(v: string | undefined): v is Locale {
  return locales.some((l) => l === v);
}
