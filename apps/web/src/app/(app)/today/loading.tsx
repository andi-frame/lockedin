import { getTranslations } from "next-intl/server";

// Shown while the server gathers the day. The band and two lines hold the shape of the page so it
// does not jump when the content arrives.
export default async function Loading() {
  const t = await getTranslations("Common");
  return (
    <div aria-busy="true" aria-live="polite">
      <span className="sr-only">{t("loading")}</span>
      <div className="h-36 rounded-panel bg-cover/90" aria-hidden />
      <div className="mt-8 grid gap-10 lg:grid-cols-2" aria-hidden>
        <div className="flex flex-col gap-3">
          <div className="h-6 w-2/3 rounded-control bg-sunken motion-safe:animate-pulse" />
          <div className="h-16 rounded-control bg-sunken motion-safe:animate-pulse" />
        </div>
        <div className="flex flex-col gap-3">
          <div className="h-6 w-1/2 rounded-control bg-sunken motion-safe:animate-pulse" />
          <div className="h-24 rounded-control bg-sunken motion-safe:animate-pulse" />
        </div>
      </div>
    </div>
  );
}
