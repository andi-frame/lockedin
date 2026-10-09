import type { Metadata, Viewport } from "next";
import { GeistMono } from "geist/font/mono";
import { GeistSans } from "geist/font/sans";
import { NextIntlClientProvider } from "next-intl";
import { cookies } from "next/headers";
import { getLocale, getTranslations } from "next-intl/server";
import type { ReactNode } from "react";
import { parseTheme, THEME_COOKIE } from "@/lib/theme";
import { Providers } from "./providers";
import "./globals.css";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Meta");
  return { title: { default: t("title"), template: "%s · Tepati" }, description: t("description") };
}

// The browser chrome follows the page ground (the contract's #F1F5F1, and the desk-lamp ground in the
// dark theme). A theme the person chose in Pengaturan overrides the system preference, so then the
// colour is that theme's alone.
const GROUND = { light: "#F1F5F1", dark: "#071516" } as const;
export async function generateViewport(): Promise<Viewport> {
  const theme = parseTheme((await cookies()).get(THEME_COOKIE)?.value);
  if (theme === "light" || theme === "dark") return { themeColor: GROUND[theme] };
  return {
    themeColor: [
      { media: "(prefers-color-scheme: light)", color: GROUND.light },
      { media: "(prefers-color-scheme: dark)", color: GROUND.dark },
    ],
  };
}

export default async function RootLayout({ children }: { children: ReactNode }) {
  const locale = await getLocale();
  const theme = parseTheme((await cookies()).get(THEME_COOKIE)?.value);
  return (
    <html lang={locale} data-theme={theme === "system" ? undefined : theme} className={`${GeistSans.variable} ${GeistMono.variable}`}>
      <body>
        <NextIntlClientProvider>
          <Providers>{children}</Providers>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
