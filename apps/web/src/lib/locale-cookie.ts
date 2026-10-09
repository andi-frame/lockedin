import { localeCookie, type Locale } from "@/i18n/config";

/** The `document.cookie` string that makes this browser show `locale` (read by i18n/request.ts). */
export const localeCookieString = (locale: Locale): string => `${localeCookie}=${locale}; path=/; max-age=31536000; samesite=lax`;

/** Client only: the account's language is what emails use, the cookie is what this browser shows. */
export function setLocaleCookie(locale: Locale): void {
  document.cookie = localeCookieString(locale);
}
