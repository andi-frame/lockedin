import { cookies } from "next/headers";
import { getRequestConfig } from "next-intl/server";

export const locales = ["id", "en"] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = "id";
export const localeCookie = "tepati_locale";

export function isLocale(v: string | undefined): v is Locale {
  return locales.some((l) => l === v);
}

// No locale in the URL (ARCHITECTURE §7): the choice lives in a cookie, and Indonesian is the default.
export default getRequestConfig(async () => {
  const stored = (await cookies()).get(localeCookie)?.value;
  const locale = isLocale(stored) ? stored : defaultLocale;
  return {
    locale,
    messages: (await import(`../../messages/${locale}.json`)).default,
  };
});
