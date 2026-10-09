import { cookies } from "next/headers";
import { getRequestConfig } from "next-intl/server";
import { defaultLocale, isLocale, localeCookie } from "./config";

// No locale in the URL (ARCHITECTURE §7): the choice lives in a cookie, and Indonesian is the default.
export default getRequestConfig(async () => {
  const stored = (await cookies()).get(localeCookie)?.value;
  const locale = isLocale(stored) ? stored : defaultLocale;
  return {
    locale,
    // Pages that show a time pass the person's own `timeZone` (Settings); this is only the fallback.
    timeZone: "Asia/Jakarta",
    messages: (await import(`../../messages/${locale}.json`)).default,
  };
});
