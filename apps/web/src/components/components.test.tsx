import { describe, expect, test } from "bun:test";
import { NextIntlClientProvider } from "next-intl";
import type { ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import en from "../../messages/en.json";
import id from "../../messages/id.json";
import { Amount } from "./amount";
import { Countdown } from "./countdown";
import { MemberLine } from "./member-line";
import { checkInStatuses, StatusChip } from "./status-chip";

const html = (ui: ReactElement, locale: "id" | "en" = "id") =>
  renderToStaticMarkup(
    <NextIntlClientProvider locale={locale} messages={locale === "id" ? id : en} timeZone="Asia/Jakarta">
      {ui}
    </NextIntlClientProvider>,
  );

// Text content with each element boundary read as a space (the layout spaces them with CSS gap), sr-only text included because assistive tech reads it.
const text = (markup: string) => markup.replace(/<[^>]+>/g, " ").replace(/\s+/g, " ").trim();

// The clock prints one character per cell, so compare it without the cell gaps.
const clock = (markup: string) => text(markup).replace(/ /g, "");

describe("Amount", () => {
  test("a debit shows a minus, the figure, the unit, the D mark and the word for screen readers", () => {
    const out = html(<Amount coins={1500} direction="debit" />);
    expect(text(out)).toBe("debit − 1.500 koin D");
    expect(out).toContain("text-debit");
    expect(out).toContain("sr-only");
  });

  test("a credit shows a plus and the K mark", () => {
    expect(text(html(<Amount coins={250} direction="credit" />))).toBe("kredit + 250 koin K");
  });

  test("a balance has no sign and no D/K mark", () => {
    expect(text(html(<Amount coins={1000} direction="balance" />))).toBe("1.000 koin");
  });

  test("a negative balance gets a minus", () => {
    expect(text(html(<Amount coins={-40} direction="balance" />))).toBe("− 40 koin");
  });

  test("the Rupiah equivalent appears when a rate is given", () => {
    expect(text(html(<Amount coins={1000} direction="balance" rate={1000} />))).toBe("1.000 koin ≈ Rp1.000.000");
  });

  test("the English unit and credit mark", () => {
    expect(text(html(<Amount coins={5} direction="credit" />, "en"))).toBe("credit + 5 coins C");
  });

  test("the sign is hidden from assistive tech, the word is not", () => {
    const out = html(<Amount coins={5} direction="debit" />);
    expect(out).toContain('aria-hidden="true">−');
  });
});

describe("StatusChip", () => {
  test("there is a chip for every check-in status in the API", () => {
    expect([...checkInStatuses].sort() as string[]).toEqual(
      ["approved", "auto_approved", "disputed", "missed", "open", "rejected", "rest", "submitted"].sort(),
    );
  });

  for (const status of checkInStatuses) {
    test(`${status} has a label and its own icon`, () => {
      const out = html(<StatusChip status={status} />);
      expect(text(out)).toBe(id.Status[status]);
      expect(out).toContain("<svg");
    });
  }

  test("every status has a different icon and label, so colour is never the only signal", () => {
    const icons = new Set(checkInStatuses.map((s) => html(<StatusChip status={s} />).match(/<svg.*<\/svg>/)?.[0]));
    expect(icons.size).toBe(checkInStatuses.length);
    expect(new Set(Object.values(id.Status)).size).toBe(checkInStatuses.length);
  });
});

describe("MemberLine", () => {
  test("prints the name and monogram, and the role when given", () => {
    const out = html(<MemberLine name="Sari Wulandari" slot={0} role="doer" />);
    expect(text(out)).toBe("SW Sari Wulandari Pelaku");
  });

  test("the two slots use different colours", () => {
    const a = html(<MemberLine name="A" slot={0} />);
    const b = html(<MemberLine name="B" slot={1} />);
    expect(a).toContain("bg-member-a");
    expect(b).toContain("bg-member-b");
  });
});

describe("Countdown", () => {
  const until = "2026-10-08T22:00:00Z";

  test("server render shows real digits when the server's clock is passed", () => {
    const out = html(<Countdown until={until} serverNow="2026-10-08T18:18:48Z" />);
    expect(out).toContain("lagi");
    expect(clock(out)).toContain("03:41:12");
  });

  test("every character of the clock sits in its own cell (fixed-cell digits)", () => {
    const out = html(<Countdown until={until} serverNow="2026-10-08T18:18:48Z" />);
    expect(out.match(/w-\[1\.2ch\]/g)?.length).toBe(6);
  });

  test("the visible digits are hidden from assistive tech and one polite live region speaks", () => {
    const out = html(<Countdown until={until} serverNow="2026-10-08T18:18:48Z" />);
    expect(out).toContain('aria-live="polite"');
    expect(out.match(/aria-live/g)?.length).toBe(1);
    expect(out).toContain("3 jam 42 menit lagi");
  });

  test("after the deadline it reads 00:00:00 and says it is over", () => {
    const out = html(<Countdown until={until} serverNow="2026-10-08T22:00:05Z" />);
    expect(clock(out)).toContain("00:00:00");
    expect(text(out)).toContain("lewat");
    expect(text(out)).toContain("Batas waktu sudah lewat");
  });

  test("without a server clock the first paint is a placeholder, never a wrong time", () => {
    expect(clock(html(<Countdown until={until} />))).toContain("--:--:--");
  });

  test("the cover variant uses light digits", () => {
    expect(html(<Countdown until={until} serverNow="2026-10-08T18:18:48Z" onCover />)).toContain("text-cover-ink");
  });
});

describe("messages", () => {
  const keys = (o: unknown, prefix = ""): string[] =>
    o && typeof o === "object"
      ? Object.entries(o).flatMap(([k, v]) => keys(v, prefix ? `${prefix}.${k}` : k))
      : [prefix];

  test("no key contains a dot, which next-intl reads as nesting", () => {
    const flat = (o: unknown): string[] =>
      o && typeof o === "object" ? Object.entries(o).flatMap(([k, v]) => [k, ...flat(v)]) : [];
    expect(flat(id).filter((k) => k.includes("."))).toEqual([]);
    expect(flat(en).filter((k) => k.includes("."))).toEqual([]);
  });

  test("en.json mirrors every key of id.json", () => {
    expect(keys(en).sort()).toEqual(keys(id).sort());
  });
});
