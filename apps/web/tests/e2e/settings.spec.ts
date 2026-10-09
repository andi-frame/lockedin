import { expect, test, type Page } from "@playwright/test";

const freshEmail = () => `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.test`;

// The toast is announced twice (visible text and a live region), so the tests wait on the request itself.
const saved = (page: Page) => page.waitForResponse((r) => r.url().endsWith("/api/v1/me") && r.request().method() === "PATCH" && r.ok());

async function register(page: Page) {
  await page.goto("/register");
  await page.getByLabel("Nama", { exact: true }).fill("Sari E2E");
  await page.getByLabel("Email", { exact: true }).fill(freshEmail());
  await page.getByLabel("Kata sandi", { exact: true }).fill("kata-sandi-e2e-1");
  await page.getByRole("button", { name: "Buat akun" }).click();
  await expect(page).toHaveURL(/\/today$/);
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
