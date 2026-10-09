import { Coins } from "@phosphor-icons/react/dist/ssr";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { buttonVariants } from "@/components/ui/button";
import { Wordmark } from "@/components/wordmark";
import { SamplePage } from "./sample-page";

const steps = [
  ["stepFirst", "stepFirstBody"],
  ["stepDaily", "stepDailyBody"],
  ["stepReview", "stepReviewBody"],
  ["stepLast", "stepLastBody"],
] as const;

const rules = [
  ["ruleOwnTitle", "ruleOwnBody"],
  ["ruleDisputeTitle", "ruleDisputeBody"],
  ["ruleOverrideTitle", "ruleOverrideBody"],
  ["ruleRestTitle", "ruleRestBody"],
  ["rulePotTitle", "rulePotBody"],
  ["ruleBookTitle", "ruleBookBody"],
] as const;

/**
 * The page at `/` for a visitor without a session (surface brief apps-web-src-app-page-tsx): the
 * passbook cover with the hook and the way in, an example month stamped over its edge, how a
 * contract runs as dated lines, the rules that protect both people, and the same button again.
 */
export function LandingPage() {
  const t = useTranslations("Landing");
  const tc = useTranslations("Nav");
  return (
    <div className="min-h-dvh bg-ground text-ink">
      <a href="#how" className="sr-only z-50 rounded-control bg-surface px-3 py-2 font-medium text-ink focus:not-sr-only focus:fixed focus:left-3 focus:top-3">
        {tc("skip")}
      </a>

      <header className="guilloche bg-cover text-cover-ink">
        <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 sm:px-8">
          <Wordmark />
          <Link href="/login" className={buttonVariants({ variant: "onCoverQuiet", size: "sm" })}>
            {t("login")}
          </Link>
        </div>

        <div className="mx-auto grid max-w-6xl gap-10 px-4 pt-8 sm:px-8 lg:grid-cols-[minmax(0,1fr)_minmax(0,35rem)] lg:gap-14 lg:pt-16">
          <div className="pb-4 lg:pb-52">
            <h1 className="max-w-[16ch] text-4xl font-semibold leading-[1.05] tracking-tight text-balance sm:text-5xl lg:text-6xl">{t("heroTitle")}</h1>
            <p className="mt-5 max-w-prose text-[17px] leading-7 text-cover-muted">{t("heroBody")}</p>
            <p className="mt-5 flex max-w-prose items-start gap-3 rounded-control bg-cover-ink/10 px-3 py-2.5 text-[15px]">
              <Coins aria-hidden weight="bold" className="mt-0.5 size-5 shrink-0" />
              {t("money")}
            </p>
            <div className="mt-6 flex flex-col gap-3 sm:flex-row">
              <Link href="/register" className={buttonVariants({ variant: "onCover", block: false, className: "w-full sm:w-auto" })}>
                {t("register")}
              </Link>
              <Link href="/login" className={buttonVariants({ variant: "onCoverQuiet", className: "w-full sm:w-auto" })}>
                {t("login")}
              </Link>
            </div>
          </div>
          <SamplePage className="relative z-10 -mb-24 lg:-mb-44" />
        </div>
      </header>

      <main>
        <section id="how" className="mx-auto max-w-6xl px-4 pb-16 pt-32 sm:px-8 lg:pt-56">
          <h2 className="text-2xl font-semibold tracking-tight">{t("howTitle")}</h2>
          <ol className="mt-6 max-w-3xl divide-y divide-rule border-y border-rule">
            {steps.map(([label, body]) => (
              <li key={label} className="grid gap-1 py-4 sm:grid-cols-[10rem_1fr] sm:gap-6">
                <span className="font-mono text-[15px] font-medium">{t(label)}</span>
                <span className="text-[15px] leading-7 text-muted">{t(body)}</span>
              </li>
            ))}
          </ol>
        </section>

        <section className="mx-auto max-w-6xl px-4 pb-20 sm:px-8">
          <h2 className="text-2xl font-semibold tracking-tight">{t("rulesTitle")}</h2>
          <dl className="mt-6 grid gap-x-12 border-t border-rule md:grid-cols-2">
            {rules.map(([title, body]) => (
              <div key={title} className="border-b border-rule py-4">
                <dt className="text-[15px] font-semibold">{t(title)}</dt>
                <dd className="mt-1 text-[15px] leading-7 text-muted">{t(body)}</dd>
              </div>
            ))}
          </dl>
        </section>
      </main>

      <section className="guilloche bg-cover text-cover-ink">
        <div className="mx-auto flex max-w-6xl flex-col gap-6 px-4 py-14 sm:px-8 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 className="text-3xl font-semibold tracking-tight text-balance">{t("closeTitle")}</h2>
            <p className="mt-2 max-w-prose text-[17px] text-cover-muted">{t("closeBody")}</p>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row">
            <Link href="/register" className={buttonVariants({ variant: "onCover", className: "w-full sm:w-auto" })}>
              {t("register")}
            </Link>
            <Link href="/login" className={buttonVariants({ variant: "onCoverQuiet", className: "w-full sm:w-auto" })}>
              {t("login")}
            </Link>
          </div>
        </div>
        <div className="border-t border-cover-ink/15">
          <div className="mx-auto flex max-w-6xl items-center justify-between px-4 py-4 text-sm text-cover-muted sm:px-8">
            <Wordmark className="text-base" />
          </div>
        </div>
      </section>
    </div>
  );
}
