import { expect, test, type Browser, type Page } from "@playwright/test";

// Signs in as a seeded backer (and, for the dispute and the override, as the doer in a second
// browser context). `tepatictl seed --scenario review` leaves the backer three proofs: two
// submitted, and one the clock has already auto-approved, inside its override window.
const password = process.env.E2E_TODAY_PASSWORD;
const emails = (isMobile: boolean) =>
  isMobile
    ? { backer: process.env.E2E_REVIEW_BACKER_MOBILE, doer: process.env.E2E_REVIEW_DOER_MOBILE }
    : { backer: process.env.E2E_REVIEW_BACKER_DESKTOP, doer: process.env.E2E_REVIEW_DOER_DESKTOP };

test.beforeEach(({ isMobile }) => {
  const e = emails(isMobile);
  test.skip(!e.backer || !e.doer || !password, "run through `bun run test:e2e`, which seeds the review scenario");
});

async function signIn(page: Page, email: string | undefined) {
  await page.goto("/login");
  await page.getByLabel("Email", { exact: true }).fill(email ?? "");
  await page.getByLabel("Kata sandi", { exact: true }).fill(password ?? "");
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page).toHaveURL(/\/today$/);
}

async function asDoer(browser: Browser, isMobile: boolean) {
  const context = await browser.newContext(isMobile ? { viewport: { width: 390, height: 844 }, isMobile: true, hasTouch: true } : {});
  const page = await context.newPage();
  await signIn(page, emails(isMobile).doer);
  return page;
}

/** Opens the check-in of a pact (by its title on the review queue, or the Today list) as the viewer. */
const openFromQueue = async (page: Page, title: string) => {
  await page.goto("/review");
  await page.getByTestId("review-queue").getByRole("link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/pacts\/[0-9a-f-]{36}\/days\/\d{4}-\d{2}-\d{2}\?of=/);
};

const status = (page: Page, label: string) => page.getByRole("main").locator("header").getByText(label, { exact: true });

test("the queue lists what waits, and approving one prints it into the history", async ({ page, isMobile }) => {
  await signIn(page, emails(isMobile).backer);
  await page.goto("/review");
  const queue = page.getByTestId("review-queue");
  await expect(queue.getByRole("link")).toHaveCount(2);
  await expect(queue.getByText("Review: approve")).toBeVisible();
  await expect(queue.getByText("5 kata").first()).toBeVisible();
  await expect(queue.getByText("Batas tinjauan").first()).toBeVisible();

  await openFromQueue(page, "Review: approve");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("Setoran Bima (tinjau)");
  await expect(status(page, "Menunggu tinjauan")).toBeVisible();
  await expect(page.getByText("Selesai latihan soal hari ini.")).toBeVisible(); // the proof, read-only
  await expect(page.getByTestId("timeline")).toContainText("Bima (tinjau) mengirim bukti");

  // A click that lands before the page has hydrated does nothing, so it is repeated until the
  // decision is taken (once taken, the button is gone and nothing is clicked twice).
  await expect(async () => {
    await page.getByRole("button", { name: "Setujui", exact: true }).click({ timeout: 2000 });
    await expect(page.getByText("Bukti disetujui.", { exact: true })).toBeVisible({ timeout: 3000 });
  }).toPass({ timeout: 20_000 });
  await expect(status(page, "Disetujui")).toBeVisible();
  await expect(page.getByTestId("timeline")).toContainText("Kamu menyetujui bukti");
  await expect(page.getByTestId("decision-actions")).toHaveCount(0);

  await page.goto("/review");
  await expect(page.getByTestId("review-queue").getByRole("link")).toHaveCount(1);
});

test("reject needs a reason, the doer disputes, and the backer's ruling shows as power", async ({ page, browser, isMobile }) => {
  test.setTimeout(120_000);
  await signIn(page, emails(isMobile).backer);
  await openFromQueue(page, "Review: dispute");
  const dayUrl = page.url().replace(/\?of=.*$/, "");

  // Rejecting asks for a written reason of at least ten characters, and says how many are missing.
  await page.getByRole("button", { name: "Tolak", exact: true }).click();
  const dialog = page.getByRole("dialog");
  const confirm = dialog.getByRole("button", { name: "Tolak", exact: true });
  await expect(confirm).toBeDisabled();
  await dialog.getByLabel("Alasan").fill("kurang");
  await expect(dialog.getByText("Kurang 4 karakter.")).toBeVisible();
  await expect(confirm).toBeDisabled();
  await dialog.getByLabel("Alasan").fill("Fotonya bukan soal hari ini");
  await expect(confirm).toBeEnabled();
  await confirm.click();
  await expect(page.getByText("Bukti ditolak.", { exact: true })).toBeVisible();
  await expect(status(page, "Ditolak")).toBeVisible();
  await expect(page.getByTestId("timeline")).toContainText("Fotonya bukan soal hari ini");

  // The doer sees the rejection and its reason, and disputes it.
  const doer = await asDoer(browser, isMobile);
  await doer.goto(dayUrl);
  await expect(status(doer, "Ditolak")).toBeVisible();
  await expect(doer.getByTestId("timeline")).toContainText("Fotonya bukan soal hari ini");
  await expect(doer.getByRole("link", { name: "Kirim ulang bukti" })).toBeVisible(); // still before the deadline
  await doer.getByRole("button", { name: "Ajukan keberatan" }).click();
  await doer.getByRole("dialog").getByLabel("Alasan").fill("Foto itu soal integral, sesuai target hari ini");
  await doer.getByRole("dialog").getByRole("button", { name: "Ajukan keberatan" }).click();
  await expect(doer.getByText("Keberatan diajukan.", { exact: true })).toBeVisible();
  await expect(status(doer, "Disanggah")).toBeVisible();
  await expect(doer.getByTestId("decision-actions")).toHaveCount(0); // the backer decides now

  // The backer rules, with a rationale both can read; the ruling is drawn as backer power.
  await page.reload();
  await expect(status(page, "Disanggah")).toBeVisible();
  await page.getByRole("button", { name: "Putuskan keberatan" }).click();
  const ruling = page.getByRole("dialog");
  await ruling.getByLabel("Terima keberatan, bukti disetujui").check();
  await ruling.getByLabel("Alasan").fill("Setelah dilihat lagi, fotonya memang soal integral");
  await ruling.getByRole("button", { name: "Putuskan keberatan" }).click();
  await expect(page.getByText("Keberatan diputuskan.", { exact: true })).toBeVisible();
  await expect(status(page, "Disetujui")).toBeVisible();
  const power = page.getByTestId("timeline").locator('[data-tone="power"]');
  await expect(power).toHaveCount(1);
  await expect(power).toContainText("Kamu menerima keberatan");
  await expect(power).toContainText("Kuasa penyokong");
  await expect(power).toContainText("memang soal integral");

  await doer.reload();
  await expect(status(doer, "Disetujui")).toBeVisible();
  await expect(doer.getByTestId("timeline").locator('[data-tone="power"]')).toContainText("Andi (tinjau) menerima keberatan");
  await doer.context().close();
});

test("an override shows the remaining count, and the doer sees who cancelled it and why", async ({ page, browser, isMobile }) => {
  test.setTimeout(120_000);
  await signIn(page, emails(isMobile).backer);
  // The auto-approved proof is no longer in the queue; it opens from the pact's calendar.
  await page.goto("/pacts");
  await page.getByRole("link", { name: "Review: override" }).first().click();
  await page.getByTestId("calendar").locator("td[data-today] a").click();
  await expect(page).toHaveURL(/\/days\/\d{4}-\d{2}-\d{2}/);
  const dayUrl = page.url().replace(/\?of=.*$/, "");
  await expect(status(page, "Disetujui otomatis")).toBeVisible();
  await expect(page.getByTestId("timeline")).toContainText("Sistem menyetujui otomatis");

  await page.getByRole("button", { name: "Batalkan persetujuan" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByText("Sisa pembatalan di kontrak ini: 3")).toBeVisible();
  await dialog.getByLabel("Alasan").fill("Lampirannya bukan hasil kerjamu sendiri");
  await dialog.getByRole("button", { name: "Batalkan persetujuan" }).click();
  await expect(page.getByText("Persetujuan dibatalkan.", { exact: true })).toBeVisible();
  await expect(status(page, "Ditolak")).toBeVisible();
  await expect(page.getByRole("note")).toContainText("Dibatalkan oleh penyokong");
  await expect(page.getByTestId("timeline").locator('[data-tone="power"]')).toContainText("Kuasa penyokong");

  // The doer sees it too, with the reason, and has nothing to dispute with.
  const doer = await asDoer(browser, isMobile);
  await doer.goto(dayUrl);
  const note = doer.getByRole("note");
  await expect(note).toContainText("Dibatalkan oleh penyokong");
  await expect(note).toContainText("Lampirannya bukan hasil kerjamu sendiri");
  await expect(note).toContainText("tidak bisa diajukan keberatan");
  await expect(doer.getByRole("button", { name: "Ajukan keberatan" })).toHaveCount(0);
  await doer.context().close();
});
