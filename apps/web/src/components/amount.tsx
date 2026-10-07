import { useTranslations } from "next-intl";
import { cn } from "@/lib/cn";
import { formatCoins, formatIdr, signSymbol, type Direction } from "@/lib/format";

// Fixed sizes per step instead of `em` ratios: an em-based unit or D/K mark falls under 11px at
// the small step, where it can no longer be read.
const sizes = {
  sm: { figure: "text-sm", unit: "text-xs", mark: "text-[11px]", rate: "text-xs" },
  md: { figure: "text-base", unit: "text-[13px]", mark: "text-[11px]", rate: "text-[13px]" },
  lg: { figure: "text-2xl", unit: "text-sm", mark: "text-xs", rate: "text-sm" },
  xl: { figure: "text-4xl", unit: "text-base", mark: "text-sm", rate: "text-base" },
} as const;

const tones: Record<Direction, string> = {
  debit: "text-debit",
  credit: "text-credit",
  balance: "text-ink",
};

/**
 * A coin amount as the passbook prints it: a sign, the figure in mono, a D/K mark, the unit, and
 * optionally the Rupiah equivalent. Colour is never the only signal (PRODUCT: accessibility), so
 * debit/credit also carry the sign and the mark, and a screen reader hears the word.
 */
export function Amount({
  coins,
  direction,
  rate,
  size = "md",
  className,
}: {
  coins: number;
  direction: Direction;
  /** Rupiah per coin (`coin_rate_idr`). When given, the equivalent is shown under the figure. */
  rate?: number;
  size?: keyof typeof sizes;
  className?: string;
}) {
  const t = useTranslations("Amount");
  // A balance has no side; only a negative one needs a sign.
  const sign = direction === "balance" ? (coins < 0 ? signSymbol("debit") : "") : signSymbol(direction);
  const mark = direction === "debit" ? t("debitMark") : direction === "credit" ? t("creditMark") : null;
  const word = direction === "debit" ? t("debit") : direction === "credit" ? t("credit") : null;

  return (
    <span className={cn("inline-flex flex-col", sizes[size].figure, className)}>
      <span className={cn("inline-flex items-baseline gap-[0.4em] font-mono font-medium leading-tight", tones[direction])}>
        {word ? <span className="sr-only">{word} </span> : null}
        {sign ? <span aria-hidden>{sign}</span> : null}
        <span>{formatCoins(coins)}</span>
        <span className={cn("font-sans font-normal text-muted", sizes[size].unit)}>{t("unit")}</span>
        {mark ? (
          <span
            aria-hidden
            className={cn(
              "self-center rounded-control border border-current px-1 py-0.5 font-semibold leading-none",
              sizes[size].mark,
            )}
          >
            {mark}
          </span>
        ) : null}
      </span>
      {rate ? (
        <span className={cn("font-mono leading-tight text-muted", sizes[size].rate)}>
          {t("approx")} {formatIdr(coins, rate)}
        </span>
      ) : null}
    </span>
  );
}
