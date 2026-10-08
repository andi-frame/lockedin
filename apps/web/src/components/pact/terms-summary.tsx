import { useLocale, useTranslations } from "next-intl";
import type { components } from "@/lib/api/schema";
import { termsSummary } from "@/lib/pact/summary";

type Terms = components["schemas"]["Terms"];

/**
 * The plain-language terms (SPEC §4), one sentence per ruled line like a passbook page. It reads
 * the same Terms the hash covers, so the person signs exactly what they read.
 */
export function TermsSummary({ terms, names }: { terms: Terms; names: { backer: string; doer: string } }) {
  const t = useTranslations();
  const locale = useLocale() === "en" ? "en" : "id";
  const lines = termsSummary(terms, { locale, names });
  return (
    <ul className="divide-y divide-rule border-y border-rule">
      {lines.map((line, i) => (
        <li key={`${line.key}-${i}`} className="py-3 text-[15px] leading-6 text-ink">
          {t(line.key, line.params)}
        </li>
      ))}
    </ul>
  );
}
