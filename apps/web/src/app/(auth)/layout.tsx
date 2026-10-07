import { getTranslations } from "next-intl/server";
import type { ReactNode } from "react";
import { Wordmark } from "@/components/wordmark";

// Outside the shell: no rail, no tab bar. The cover band carries the passbook identity; on a
// phone it is a short strip above the form, on desktop it takes the left side.
export default async function AuthLayout({ children }: { children: ReactNode }) {
  const t = await getTranslations("Auth");
  return (
    <div className="min-h-dvh lg:grid lg:grid-cols-[5fr_7fr]">
      <header className="guilloche bg-cover px-4 py-5 text-cover-ink sm:px-8 lg:flex lg:flex-col lg:justify-between lg:px-14 lg:py-14">
        <Wordmark />
        <div className="mt-6 max-w-sm lg:mt-0">
          <p className="text-2xl font-semibold leading-tight tracking-tight text-balance lg:text-4xl">{t("brandLine")}</p>
          <p className="mt-3 hidden text-cover-muted lg:block">{t("brandBody")}</p>
        </div>
      </header>
      <main id="main" className="flex justify-center px-4 py-10 sm:px-8 lg:items-center lg:py-14">
        <div className="w-full max-w-sm">{children}</div>
      </main>
    </div>
  );
}
