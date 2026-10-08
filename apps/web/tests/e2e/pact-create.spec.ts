import { expect, test, type Browser, type Page } from "@playwright/test";

const freshEmail = () => `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.test`;
const PASSWORD = "kata-sandi-e2e-1";

async function fillRegister(page: Page, name: string) {
  await page.getByLabel("Nama", { exact: true }).fill(name);
  await page.getByLabel("Email", { exact: true }).fill(freshEmail());
  await page.getByLabel("Kata sandi", { exact: true }).fill(PASSWORD);
  await page.getByRole("button", { name: "Buat akun" }).click();
}

async function person(browser: Browser, baseURL: string | undefined) {
  const context = await browser.newContext({ baseURL, viewport: { width: 1440, height: 900 } });
  return { context, page: await context.newPage() };
}

async function sign(page: Page, name: string, button: string) {
  await page.getByRole("checkbox", { name: /Aku sudah membaca ketentuan/ }).check();
  await page.getByLabel("Ketik namamu untuk menandatangani").fill(name);
  await page.getByRole("button", { name: button, exact: true }).click();
}

// Registering costs one auth request each and the API allows 10 a minute per IP, so this runs on
// the desktop project only and registers each person exactly once.
test("A proposes, B joins from the invite link and signs, an edit clears both signatures, both sign again and the pact is scheduled", async ({
  browser,
  baseURL,
  isMobile,
}) => {
  test.skip(isMobile, "desktop project only (auth rate limit)");
  test.setTimeout(Math.max(90_000, Number(process.env.E2E_TIMEOUT_MS ?? 0))); // two people, four screens of wizard, two sign-offs
  const A = await person(browser, baseURL);
  const B = await person(browser, baseURL);
  const a = A.page;
  const b = B.page;

  // --- A registers and walks the wizard.
  await a.goto("/register");
  await fillRegister(a, "Andi E2E");
  await expect(a).toHaveURL(/\/today$/);
  await a.goto("/pacts");
  await expect(a.getByText("Belum ada kontrak.")).toBeVisible();
  await a.getByRole("link", { name: "Buat kontrak" }).click();
  await expect(a).toHaveURL(/\/pacts\/new$/);

  // Step 1: the form refuses to move on without a name, and says why.
  await a.getByRole("button", { name: "Lanjut" }).click();
  await expect(a.getByText("Nama kontrak belum diisi.")).toBeVisible();
  await expect(a.getByLabel("Nama kontrak")).toBeFocused();
  await a.getByLabel("Nama kontrak").fill("Kalkulus sebulan");
  await a.getByRole("button", { name: "Lanjut" }).click();

  // Step 2: commitments, one per member.
  await expect(a.getByRole("heading", { name: "Komitmen masing-masing" })).toBeFocused();
  await a.getByRole("button", { name: "Lanjut" }).click();
  await expect(a.getByText("Tulis komitmen hariannya.").first()).toBeVisible();
  await a.locator('[name="backer.commitment"]').fill("Belajar Kalkulus minimal 2 jam");
  await a.locator('[name="doer.commitment"]').fill("Latihan soal UTBK 50 soal");
  await a.getByRole("button", { name: "Lanjut" }).click();

  // Step 3: defaults are fine, and a pot of 1.000 coins is shown in Rupiah.
  await expect(a.getByRole("heading", { name: "Koin dan aturan" })).toBeVisible();
  await expect(a.getByText("Setara Rp1.000.000")).toBeVisible();
  await a.getByRole("button", { name: "Lanjut" }).click();

  // Step 4: the plain-language terms (SPEC §4), then propose.
  await expect(a.getByRole("heading", { name: "Tinjau ketentuan", level: 2 })).toBeVisible();
  await expect(a.getByText("Kalau Pelaku melewatkan 1 hari, pot berkurang 50 koin (Rp50.000).")).toBeVisible();
  await expect(a.getByText("Kalau Andi E2E melewatkan 1 hari, pot bertambah 50 koin (Rp50.000)")).toBeVisible();
  await a.getByRole("button", { name: "Ajukan kontrak" }).click();

  await expect(a.getByRole("heading", { name: "Kontrak diajukan" })).toBeVisible();
  const link = await a.getByLabel("Tautan undangan").inputValue();
  expect(link).toMatch(/\/invite\/[A-Za-z0-9_-]+$/);
  await a.getByRole("link", { name: "Buka kontrak" }).click();
  await expect(a.getByRole("heading", { level: 1, name: "Kalkulus sebulan" })).toBeVisible();
  await expect(a.getByText("Menunggu pelaku bergabung lewat tautan undangan.")).toBeVisible();
  const pactUrl = a.url();

  // --- B opens the link signed out, is sent to log in, registers, and lands back on the invite.
  const invitePath = new URL(link).pathname;
  await b.goto(invitePath);
  await expect(b).toHaveURL(new RegExp(`/login\\?next=${encodeURIComponent(invitePath).replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`));
  await b.getByRole("link", { name: "Buat akun" }).click();
  await fillRegister(b, "Sari E2E");
  await expect(b).toHaveURL(new RegExp(`${invitePath}$`));
  await expect(b.getByRole("heading", { level: 1, name: "Kalkulus sebulan" })).toBeVisible();
  await expect(b.getByText("Andi E2E mengajakmu membuat kontrak belajar.")).toBeVisible();
  // B reads the terms with their own name in them.
  await expect(b.getByText("Kalau Sari E2E melewatkan 1 hari, pot berkurang 50 koin (Rp50.000).")).toBeVisible();

  // Signing needs the box ticked and the exact name.
  await b.getByRole("button", { name: "Gabung dan tandatangani" }).click();
  await expect(b.getByText("Centang dulu bahwa kamu sudah membaca ketentuan.")).toBeVisible();
  await b.getByRole("checkbox", { name: /Aku sudah membaca ketentuan/ }).check();
  await b.getByLabel("Ketik namamu untuk menandatangani").fill("Sari");
  await b.getByRole("button", { name: "Gabung dan tandatangani" }).click();
  await expect(b.getByText("Namanya belum sama dengan nama di akunmu.")).toBeVisible();
  await b.getByLabel("Ketik namamu untuk menandatangani").fill("sari e2e");
  await b.getByRole("button", { name: "Gabung dan tandatangani" }).click();
  await expect(b).toHaveURL(/\/pacts\/[0-9a-f-]{36}$/);
  await expect(b.getByText("Sudah menandatangani sebagai Sari E2E")).toBeVisible();
  await expect(b.getByText("Kamu sudah menandatangani. Kontrak terjadwal begitu temanmu juga menandatangani.")).toBeVisible();

  // The same link cannot be used twice.
  await b.goto(invitePath);
  await expect(b.getByRole("heading", { name: "Undangan ini tidak bisa dipakai" })).toBeVisible();
  await b.goto(pactUrl);

  // --- A edits the terms: both signatures are cleared, visibly, for both people.
  await a.reload();
  await expect(a.getByText("Sudah menandatangani sebagai Sari E2E")).toBeVisible();
  await a.getByRole("link", { name: "Ubah ketentuan" }).first().click();
  await expect(a.getByLabel("Nama kontrak")).toHaveValue("Kalkulus sebulan");
  await a.getByRole("button", { name: "Lanjut" }).click();
  await a.getByRole("button", { name: "Lanjut" }).click();
  await a.getByLabel("Pot awal (koin)").fill("1200");
  await a.getByRole("button", { name: "Lanjut" }).click();
  await expect(a.getByText("Menyimpan perubahan mengosongkan tanda tangan kedua pihak.")).toBeVisible();
  await a.getByRole("button", { name: "Simpan perubahan" }).click();
  await expect(a).toHaveURL(/\/pacts\/[0-9a-f-]{36}\?edited=1$/);
  await expect(a.getByRole("status").filter({ hasText: "Ketentuan diubah." })).toBeVisible();
  await expect(a.getByText("Sudah menandatangani")).toHaveCount(0);
  await expect(a.getByText("Belum menandatangani")).toHaveCount(2);
  await expect(a.getByText("Pot dimulai dari 1.200 koin (Rp1.200.000)")).toBeVisible();

  await b.reload();
  await expect(b.getByText("Belum menandatangani")).toHaveCount(2);
  await expect(b.getByText("Ketentuan sudah berubah sejak terakhir kali ditandatangani")).toBeVisible();

  // --- Both sign the new version; the second signature schedules the pact.
  await sign(b, "Sari E2E", "Tandatangani");
  await expect(b.getByText("Sudah menandatangani sebagai Sari E2E")).toBeVisible();
  await a.reload();
  await sign(a, "Andi E2E", "Tandatangani");
  await expect(a.getByText("Terjadwal", { exact: true })).toBeVisible();
  await expect(a.getByText(/Kontrak terjadwal dan dimulai/)).toBeVisible();

  // The list shows it too.
  await a.goto("/pacts");
  await expect(a.getByRole("link", { name: /Kalkulus sebulan/ })).toContainText("Terjadwal");

  await A.context.close();
  await B.context.close();
});
