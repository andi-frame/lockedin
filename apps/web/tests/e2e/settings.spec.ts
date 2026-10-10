import { expect, test, type Page } from "@playwright/test";

const freshEmail = () => `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.test`;

// The toast is announced twice (visible text and a live region), so the tests wait on the request itself.
const saved = (page: Page) => page.waitForResponse((r) => r.url().endsWith("/api/v1/me") && r.request().method() === "PATCH" && r.ok());

async function register(page: Page): Promise<string> {
  const email = freshEmail();
  await page.goto("/register");
  await page.getByLabel("Nama", { exact: true }).fill("Sari E2E");
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Kata sandi", { exact: true }).fill("kata-sandi-e2e-1");
  await page.getByRole("button", { name: "Buat akun" }).click();
  await expect(page).toHaveURL(/\/today$/);
  return email;
}

test("settings: name, time zone and emails are saved and survive a reload", async ({ page }) => {
  await register(page);
  await page.goto("/settings");
  await expect(page.getByRole("heading", { level: 1, name: "Pengaturan" })).toBeVisible();
  const account = page.locator("section[aria-labelledby=account]");
  const email = page.locator("section[aria-labelledby=email]");

  // Saving with nothing changed says so instead of sending anything.
  await account.getByRole("button", { name: "Simpan" }).click();
  await expect(page.getByText("Belum ada yang berubah.", { exact: true })).toBeVisible();

  await account.getByLabel("Nama", { exact: true }).fill("Sari Dewi");
  await account.getByLabel("Zona waktu").click();
  await page.getByRole("option", { name: "Asia/Makassar" }).click();
  const nameSaved = saved(page);
  await account.getByRole("button", { name: "Simpan" }).click();
  await nameSaved;
  await expect(page.getByText("Perubahan disimpan.", { exact: true })).toBeVisible();

  // Every email is on for a new account; switch one off and keep the rest.
  for (const label of ["buktimu ditolak", "ada bukti baru yang harus kamu tinjau"]) await expect(email.getByRole("checkbox", { name: label })).toBeChecked();
  await email.getByRole("checkbox", { name: "buktimu ditolak" }).uncheck();
  const emailSaved = saved(page);
  await email.getByRole("button", { name: "Simpan" }).click();
  await emailSaved;

  await page.reload();
  await expect(account.getByLabel("Nama", { exact: true })).toHaveValue("Sari Dewi");
  await expect(account.getByLabel("Zona waktu")).toContainText("Asia/Makassar");
  await expect(email.getByRole("checkbox", { name: "buktimu ditolak" })).not.toBeChecked();
  await expect(email.getByRole("checkbox", { name: "ada bukti baru yang harus kamu tinjau" })).toBeChecked();
  await expect(page.getByText("Yang selalu dikirim:", { exact: false })).toBeVisible();
});

test("settings: an empty name is refused before anything is sent, and the language switches the app", async ({ page }) => {
  await register(page);
  await page.goto("/settings");
  const account = page.locator("section[aria-labelledby=account]");

  await account.getByLabel("Nama", { exact: true }).fill("   ");
  await account.getByRole("button", { name: "Simpan" }).click();
  await expect(account.getByText("Nama tidak boleh kosong.")).toBeVisible();

  await account.getByLabel("Nama", { exact: true }).fill("Sari E2E");
  await account.getByLabel("Bahasa").click();
  await page.getByRole("option", { name: "English" }).click();
  await account.getByRole("button", { name: "Simpan" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: "Email" })).toBeVisible();
});

test("settings: the account's language follows the person to a new device", async ({ page }) => {
  const email = await register(page);
  await page.goto("/settings");
  const account = page.locator("section[aria-labelledby=account]");
  await account.getByLabel("Bahasa").click();
  await page.getByRole("option", { name: "English" }).click();
  const done = saved(page);
  await account.getByRole("button", { name: "Simpan" }).click();
  await done;
  await expect(page.getByRole("heading", { level: 1, name: "Settings" })).toBeVisible();

  // A new device has no language cookie: sign out and forget it, as another browser would.
  await page.getByRole("button", { name: "Sign out" }).first().click();
  await expect(page).toHaveURL(/\/login$/);
  await page.evaluate(() => {
    document.cookie = "tepati_locale=; path=/; max-age=0";
  });
  await page.reload();
  await expect(page.getByRole("button", { name: "Masuk" })).toBeVisible();

  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Kata sandi", { exact: true }).fill("kata-sandi-e2e-1");
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page).toHaveURL(/\/today$/);
  await expect(page.getByRole("heading", { level: 1, name: "Today" })).toBeVisible();
});

test("settings: changing the password signs the other devices out and keeps this one", async ({ page, browser }) => {
  const email = await register(page);

  // Another device, signed in with the same account.
  const other = await browser.newContext();
  const phone = await other.newPage();
  await phone.goto("/login");
  await phone.getByLabel("Email", { exact: true }).fill(email);
  await phone.getByLabel("Kata sandi", { exact: true }).fill("kata-sandi-e2e-1");
  await phone.getByRole("button", { name: "Masuk" }).click();
  await expect(phone).toHaveURL(/\/today$/);

  await page.goto("/settings");
  const section = page.locator("section[aria-labelledby=password]");

  // Client checks come first, and the server refuses a wrong current password at the field.
  await section.getByRole("button", { name: "Ganti kata sandi" }).click();
  await expect(section.getByText("Isi kata sandimu.")).toBeVisible();
  await section.getByLabel("Kata sandi sekarang", { exact: true }).fill("bukan-kata-sandi-ini");
  await section.getByLabel("Kata sandi baru", { exact: true }).fill("kata-sandi-baru-2");
  await section.getByRole("button", { name: "Ganti kata sandi" }).click();
  await expect(section.getByText("Kata sandi yang sekarang belum cocok.")).toBeVisible();

  await section.getByLabel("Kata sandi sekarang", { exact: true }).fill("kata-sandi-e2e-1");
  const changed = page.waitForResponse((r) => r.url().endsWith("/api/v1/me/password") && r.request().method() === "POST" && r.status() === 204);
  await section.getByRole("button", { name: "Ganti kata sandi" }).click();
  await changed;
  await expect(page.getByText("Perangkat lain sudah dikeluarkan.", { exact: true })).toBeVisible();

  // This device is still in; the other one is not.
  await page.reload();
  await expect(page.getByRole("heading", { level: 1, name: "Pengaturan" })).toBeVisible();
  await phone.goto("/today");
  await expect(phone).toHaveURL(/\/login/);
  await other.close();

  // The old password is gone, the new one works.
  await page.getByRole("button", { name: "Keluar" }).first().click();
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Kata sandi", { exact: true }).fill("kata-sandi-e2e-1");
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page.getByText("Email atau kata sandi belum cocok", { exact: false })).toBeVisible();
  await page.getByLabel("Kata sandi", { exact: true }).fill("kata-sandi-baru-2");
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page).toHaveURL(/\/today$/);
});
