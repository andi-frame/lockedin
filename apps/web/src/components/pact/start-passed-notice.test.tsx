import { expect, test } from "bun:test";
import { NextIntlClientProvider } from "next-intl";
import { renderToStaticMarkup } from "react-dom/server";
import messages from "../../../messages/id.json";
import { StartPassedNotice } from "./start-passed-notice";

const render = (props: { date: string; editHref?: string }) =>
  renderToStaticMarkup(
    <NextIntlClientProvider locale="id" messages={messages} timeZone="Asia/Jakarta">
      <StartPassedNotice {...props} />
    </NextIntlClientProvider>,
  );

test("the backer is told the date has passed and gets the way to move it", () => {
  const html = render({ date: "2 November 2026", editHref: "/pacts/abc/edit" });
  expect(html).toContain("2 November 2026");
  expect(html).toContain("sudah lewat");
  expect(html).toContain('href="/pacts/abc/edit"');
  expect(html).toContain("Geser tanggalnya");
});

test("anyone else is told to ask the backer, with no link to a page they cannot use", () => {
  const html = render({ date: "2 November 2026" });
  expect(html).toContain("Minta penyokong");
  expect(html).not.toContain("href=");
});
