import type { Metadata } from "next";
import { cookies } from "next/headers";
import { getTranslations } from "next-intl/server";
import { redirect } from "next/navigation";
import { LandingPage } from "@/components/landing/landing-page";
import { HOME, SESSION_COOKIE } from "@/lib/auth/guard";
import { getCurrentUser } from "@/lib/auth/session";

export async function generateMetadata(): Promise<Metadata> {
  const t = await getTranslations("Landing");
  return { title: { absolute: t("metaTitle") }, description: t("metaDescription") };
}

// src/proxy.ts sends a visitor with a session cookie to /today. A stale cookie gets here, so the
// API is asked once; without a cookie there is nothing to ask and the page stays static copy.
export default async function Home() {
  if ((await cookies()).has(SESSION_COOKIE) && (await getCurrentUser())) redirect(HOME);
  return <LandingPage />;
}
