import { expect, test, type Browser, type Page } from "@playwright/test";

// `tepatictl seed --scenario settlement` leaves a backer and a doer with two pacts whose days are
// over and settled: `settling`, 900 coins owed, nobody has said anything yet. The first pact goes
// through "the backer marks it paid, then the doer confirms", the second through "the doer
// confirms alone" (SPEC §3: that is sufficient).
const password = process.env.E2E_TODAY_PASSWORD;
const emails = (isMobile: boolean) =>
  isMobile
    ? { backer: process.env.E2E_SETTLE_BACKER_MOBILE, doer: process.env.E2E_SETTLE_DOER_MOBILE }
    : { backer: process.env.E2E_SETTLE_BACKER_DESKTOP, doer: process.env.E2E_SETTLE_DOER_DESKTOP };

test.beforeEach(({ isMobile }) => {
  const e = emails(isMobile);
  test.skip(!e.backer || !e.doer || !password, "run through `bun run test:e2e`, which seeds the settlement scenario");
});

async function signIn(page: Page, email: string | undefined) {
  await page.goto("/login");
  await page.getByLabel("Email", { exact: true }).fill(email ?? "");
  await page.getByLabel("Kata sandi", { exact: true }).fill(password ?? "");
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page).toHaveURL(/\/today$/);
}

async function newPage(browser: Browser, isMobile: boolean, email: string | undefined) {
  const context = await browser.newContext(isMobile ? { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true } : {});
  const page = await context.newPage();
  await signIn(page, email);
  return page;
}

async function openPact(page: Page, title: string) {
  await page.goto("/pacts");
  await page.getByRole("link", { name: title }).first().click();
  await expect(page).toHaveURL(/\/pacts\/[0-9a-f-]{36}$/);
  await expect(page.getByTestId("settlement")).toBeVisible();
  return page.url();
}

const panel = (page: Page) => page.getByTestId("settlement");

// A button pressed before the page has hydrated does nothing, so the click is repeated until the
// dialog is open; once it is open nothing is clicked twice.
async function openDialog(page: Page, button: string) {
  await expect(async () => {
    await page.getByRole("button", { name: button, exact: true }).click({ timeout: 2000 });
    await expect(page.getByRole("dialog")).toBeVisible({ timeout: 2000 });
  }).toPass({ timeout: 20_000 });
}

test("the backer marks it paid with a note, then the doer confirms and the pact is complete", async ({ browser, isMobile }) => {
  test.setTimeout(Math.max(120_000, Number(process.env.E2E_TIMEOUT_MS ?? 0)));
  const e = emails(isMobile);
  const backer = await newPage(browser, isMobile, e.backer);
  const url = await openPact(backer, "Settlement: paid first");

  // The amount is the payout the worker fixed, with its Rupiah equivalent; the payout line is in the coin book and the pot is 0.
  await expect(panel(backer)).toHaveAttribute("data-phase", "settling");
  await expect(panel(backer).getByText("900", { exact: true })).toBeVisible();
  await expect(panel(backer).getByText("≈ Rp900.000")).toBeVisible();
  await expect(panel(backer).getByText("Andi (bayar) belum menandai sudah dibayar")).toBeVisible();
  await expect(backer.getByRole("button", { name: "Konfirmasi diterima" })).toHaveCount(0); // only the doer confirms
  const payoutLine = backer.locator('tr[data-kind="payout"]');
  await expect(payoutLine).toHaveCount(1);
  await expect(payoutLine.locator("td").last()).toHaveText("0");

  await openDialog(backer, "Tandai sudah dibayar");
  await backer.getByRole("dialog").getByLabel("Catatan (opsional)").fill("Transfer BCA tanggal 8");
  await backer.getByRole("dialog").getByRole("button", { name: "Tandai sudah dibayar" }).click();
  await expect(backer.getByText("Dicatat: sudah dibayar.", { exact: true })).toBeVisible();
  await expect(panel(backer).getByText("Andi (bayar) menandai sudah dibayar")).toBeVisible();
  await expect(panel(backer).getByText("Catatan: Transfer BCA tanggal 8")).toBeVisible();
  await expect(panel(backer).getByText("Menunggu Bima (bayar) mengonfirmasi.")).toBeVisible();
  await expect(backer.getByRole("button", { name: "Tandai sudah dibayar" })).toHaveCount(0);

  // The doer sees who paid and how, and confirms; the backer's mark is there, so no warning.
  const doer = await newPage(browser, isMobile, e.doer);
  await doer.goto(url);
  await expect(panel(doer).getByText("Catatan: Transfer BCA tanggal 8")).toBeVisible();
  await expect(doer.getByRole("button", { name: "Tandai sudah dibayar" })).toHaveCount(0); // only the backer marks
  await openDialog(doer, "Konfirmasi diterima");
  await expect(doer.getByRole("dialog").getByText("Konfirmasi hanya kalau uangnya memang sudah kamu terima")).toHaveCount(0);
  await doer.getByRole("dialog").getByRole("button", { name: "Konfirmasi diterima" }).click();
  await expect(doer.getByText("Kontrak selesai.", { exact: true })).toBeVisible();
  await expect(panel(doer)).toHaveAttribute("data-phase", "completed");
  await expect(panel(doer).getByRole("heading", { name: "Kontrak selesai" })).toBeVisible();
  await expect(panel(doer).getByText("Bima (bayar) mengonfirmasi sudah menerima")).toBeVisible();
  await expect(doer.getByRole("button", { name: "Konfirmasi diterima" })).toHaveCount(0);

  // The backer sees the same summary, with nothing left to press.
  await backer.reload();
  await expect(panel(backer)).toHaveAttribute("data-phase", "completed");
  await expect(panel(backer).getByRole("button")).toHaveCount(0);
  await backer.context().close();
  await doer.context().close();
});

test("the doer's confirmation alone completes the pact, after a warning that nothing was marked paid", async ({ browser, isMobile }) => {
  test.setTimeout(Math.max(120_000, Number(process.env.E2E_TIMEOUT_MS ?? 0)));
  const doer = await newPage(browser, isMobile, emails(isMobile).doer);
  await openPact(doer, "Settlement: doer only");
  await expect(panel(doer).getByText("Menunggu penyokong membayar.", { exact: false })).toBeVisible();

  await openDialog(doer, "Konfirmasi diterima");
  await expect(doer.getByRole("dialog").getByText("Penyokong belum menandai sudah dibayar. Konfirmasi hanya kalau uangnya memang sudah kamu terima.")).toBeVisible();
  await doer.getByRole("dialog").getByRole("button", { name: "Konfirmasi diterima" }).click();
  await expect(doer.getByText("Kontrak selesai.", { exact: true })).toBeVisible();
  await expect(panel(doer)).toHaveAttribute("data-phase", "completed");
  await expect(panel(doer).getByText("Andi (bayar) belum menandai sudah dibayar")).toBeVisible();
  await expect(panel(doer).getByText("Bima (bayar) mengonfirmasi sudah menerima")).toBeVisible();
  await doer.context().close();
});
