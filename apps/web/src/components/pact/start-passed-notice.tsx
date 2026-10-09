import { PencilSimple, WarningCircle } from "@phosphor-icons/react/dist/ssr";
import { useTranslations } from "next-intl";
import Link from "next/link";
import { Button } from "@/components/ui/button";

/**
 * Shown in place of the sign form once the start date has passed (SPEC §3: the server refuses
 * with `pact.start_passed`). Telling people before they type their name saves the round trip, and
 * says who can fix it: only the backer can move the dates.
 */
export function StartPassedNotice({ date, editHref }: { date: string; editHref?: string }) {
  const t = useTranslations("StartPassed");
  return (
    <div role="status" className="flex max-w-xl flex-col gap-3 rounded-panel border border-rule-strong bg-sunken px-4 py-3 text-[15px]">
      <p className="flex items-start gap-2 font-medium">
        <WarningCircle aria-hidden weight="bold" className="mt-0.5 size-5 shrink-0" />
        {t("body", { date })}
      </p>
      <p className="text-muted">{editHref ? t("backer") : t("other")}</p>
      {editHref ? (
        <div>
          <Button variant="secondary" size="sm" asChild>
            <Link href={editHref}>
              <PencilSimple aria-hidden weight="bold" className="size-4" />
              {t("edit")}
            </Link>
          </Button>
        </div>
      ) : null}
    </div>
  );
}
