import { expect, test, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";

// Signs in as its own seeded doer (the same scenario as today.spec.ts, so each spec finds the day untouched) and works on the "Today: rules" pact, whose doer
// needs at least 10 words and one ready attachment. The media worker has to be running: photos and
// videos are processed on the server, and "Siap" only appears when it has said yes.
const password = process.env.E2E_TODAY_PASSWORD;
const emailFor = (isMobile: boolean) => (isMobile ? process.env.E2E_PROOF_EMAIL_MOBILE : process.env.E2E_PROOF_EMAIL_DESKTOP);
const fixture = (name: string) => fileURLToPath(new URL(`./fixtures/${name}`, import.meta.url));

test.beforeEach(({ isMobile }) => {
  test.skip(!emailFor(isMobile) || !password, "run through `bun run test:e2e`, which seeds the today scenario");
});

async function openEditor(page: Page, isMobile: boolean) {
  await page.goto("/login");
  await page.getByLabel("Email", { exact: true }).fill(emailFor(isMobile) ?? "");
  await page.getByLabel("Kata sandi", { exact: true }).fill(password ?? "");
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page).toHaveURL(/\/today$/);
  const rules = page.locator("section").filter({ has: page.getByRole("link", { name: "Today: rules", exact: true }) });
  await rules.getByRole("link", { name: /Kirim bukti|Ubah bukti/ }).click();
  await expect(page).toHaveURL(/\/pacts\/[0-9a-f-]{36}\/days\/\d{4}-\d{2}-\d{2}$/);
}

const body = (page: Page) => page.getByRole("textbox", { name: "Bukti belajarmu" });
const send = (page: Page) => page.getByRole("button", { name: /Kirim bukti|Simpan perubahan/ });

test("a file over the limit is refused in the browser and nothing is uploaded", async ({ page, isMobile }) => {
  await openEditor(page, isMobile);
  const uploads: string[] = [];
  page.on("request", (r) => r.url().includes("/uploads") && uploads.push(r.url()));

  // A 250 MB file: built in the page, because Playwright cannot hand 250 MB to the browser.
  await page.evaluate(() => {
    const input = document.querySelector<HTMLInputElement>('input[type="file"]');
    const dt = new DataTransfer();
    dt.items.add(new File([new ArrayBuffer(250 * 1024 * 1024)], "besar.mp4", { type: "video/mp4" }));
    if (input) {
      input.files = dt.files;
      input.dispatchEvent(new Event("change", { bubbles: true }));
    }
  });
  await expect(page.getByRole("alert").filter({ hasText: "besar.mp4: terlalu besar. Batasnya 200 MB." })).toBeVisible();
  expect(uploads).toEqual([]);
  await expect(page.getByText("0 dari 1 lampiran")).toBeVisible();
});

test("a renamed file gets past the browser and is rejected by the server with a reason", async ({ page, isMobile }) => {
  await openEditor(page, isMobile);
  await page.locator('input[type="file"]').first().setInputFiles({ name: "palsu.png", mimeType: "image/png", buffer: Buffer.from("ini hanya teks biasa, bukan gambar") });
  const reason = page.getByRole("alert").filter({ hasText: "File ini tidak ikut terkirim." });
  await expect(reason).toBeVisible({ timeout: 30_000 });
  await expect(reason).toContainText(/tidak (didukung|cocok)/);
  await expect(page.getByText("0 dari 1 lampiran")).toBeVisible();
  // A rejected file is never sent with the proof, and does not count.
  await page.getByRole("button", { name: "Hapus palsu.png" }).click();
  await expect(page.getByText("palsu.png")).toHaveCount(0);
});

test("the button waits for words and ready attachments, then the proof is sent and can be edited", async ({ page, isMobile }) => {
  test.setTimeout(Math.max(120_000, Number(process.env.E2E_TIMEOUT_MS ?? 0)));
  await openEditor(page, isMobile);

  // Nothing yet: disabled, and the line under it says what is missing.
  await expect(page.getByRole("heading", { name: "Kirim bukti", level: 1 })).toBeVisible();
  await expect(send(page)).toBeDisabled();
  await expect(page.getByText("Kurang 10 kata lagi.")).toBeVisible();
  await expect(page.getByText("Kurang 1 lampiran yang siap.")).toBeVisible();

  // The toolbar offers only what the server accepts (no underline, headings 2 and 3).
  const toolbar = page.getByRole("toolbar", { name: "Format teks" });
  await expect(toolbar.getByRole("button", { name: "Tebal" })).toBeVisible();
  await expect(toolbar.getByRole("button", { name: /Garis bawah|Underline/ })).toHaveCount(0);

  // Each toolbar click is waited out (the button's pressed state) before typing: the editor takes its
  // focus back in the same transaction, and keys typed before that are lost on a slow runner.
  const bold = toolbar.getByRole("button", { name: "Tebal" });
  await body(page).click();
  await bold.click();
  await expect(bold).toHaveAttribute("aria-pressed", "true");
  await page.keyboard.type("Selesai latihan soal integral dan limit, ");
  await bold.click();
  await expect(bold).toHaveAttribute("aria-pressed", "false");
  await page.keyboard.type("benar delapan dari sepuluh");
  await expect(page.getByText("10 dari 10 kata")).toBeVisible();
  await expect(page.getByText("Kurang 10 kata lagi.")).toHaveCount(0);
  await expect(send(page)).toBeDisabled(); // still no attachment

  // A link must be an http(s) address; the message is ours, not the browser's.
  await toolbar.getByRole("button", { name: "Tautan" }).click();
  await page.getByLabel("Alamat tautan").fill("javascript:alert(1)");
  await page.getByRole("button", { name: "Pasang" }).click();
  await expect(page.getByText("Tulis alamat web yang benar")).toBeVisible();
  await page.getByRole("button", { name: "Batal", exact: true }).first().click();

  // A photo and a video go through the tray; each shows its own progress and then "Siap".
  const input = page.locator('input[type="file"]').first();
  await input.setInputFiles([fixture("photo.png"), fixture("clip.mp4")]);
  const tray = page.getByRole("list").filter({ hasText: "photo" });
  await expect(tray.getByText("Siap", { exact: true })).toHaveCount(2, { timeout: 90_000 });
  await expect(page.getByText("2 dari 1 lampiran")).toBeVisible();
  await expect(page.getByText("Semua syarat terpenuhi. Siap dikirim.")).toBeVisible();

  await expect(send(page)).toBeEnabled();
  await send(page).click();
  await expect(page).toHaveURL(/\/today$/);
  await expect(page.getByText("Bukti terkirim.", { exact: true })).toBeVisible();
  const rules = page.locator("section").filter({ has: page.getByRole("link", { name: "Today: rules", exact: true }) });
  await expect(rules.getByText("Menunggu tinjauan", { exact: true })).toBeVisible();
  await expect(rules.getByText("10 kata")).toBeVisible();

  // Until the deadline the proof can be changed: the editor comes back with what was written.
  await rules.getByRole("link", { name: "Ubah bukti" }).click();
  await expect(page.getByRole("heading", { name: "Ubah bukti", level: 1 })).toBeVisible();
  await expect(body(page)).toContainText("Selesai latihan soal integral dan limit", { timeout: 15_000 });
  await expect(page.getByText("10 dari 10 kata")).toBeVisible();
  await expect(page.getByText("Siap", { exact: true })).toHaveCount(2);
  await expect(send(page)).toBeEnabled();
});
