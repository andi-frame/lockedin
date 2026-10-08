import { expect, test, type Page } from "@playwright/test";

// A fresh address per run: the database is shared with whatever else is in it.
const freshEmail = () => `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.test`;
const PASSWORD = "kata-sandi-e2e-1";

async function register(page: Page, email: string) {
  await page.goto("/register");
  await page.getByLabel("Nama", { exact: true }).fill("Sari E2E");
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Kata sandi", { exact: true }).fill(PASSWORD);
  await page.getByRole("button", { name: "Buat akun" }).click();
  await expect(page).toHaveURL(/\/today$/);
}

// On a phone the rail is hidden and sign-out lives in Pengaturan; on desktop it is in the rail.
async function logout(page: Page, isMobile: boolean) {
  if (isMobile) await page.getByRole("link", { name: "Pengaturan" }).click();
  await page.getByRole("button", { name: "Keluar" }).first().click();
  await expect(page).toHaveURL(/\/login$/);
}

test("register lands on /today, shows the right navigation, then logout returns to /login", async ({ page, isMobile }) => {
  await register(page, freshEmail());
  await expect(page.getByRole("heading", { level: 1, name: "Hari ini" })).toBeVisible();
  await expect(page.getByText("Belum ada kontrak aktif.")).toBeVisible();

  // One nav is visible at a time (rail on desktop, tab bar on a phone), with the same four items.
  const nav = page.getByRole("navigation", { name: "Menu utama" });
  await expect(nav).toHaveCount(1);
  for (const name of ["Hari ini", "Kontrak", "Tinjau", "Pengaturan"]) await expect(nav.getByRole("link", { name })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Hari ini" })).toHaveAttribute("aria-current", "page");
  await nav.getByRole("link", { name: "Kontrak" }).click();
  await expect(page).toHaveURL(/\/pacts$/);
  await expect(nav.getByRole("link", { name: "Kontrak" })).toHaveAttribute("aria-current", "page");
  if (isMobile) await expect(nav).toHaveCSS("position", "fixed");

  await logout(page, isMobile);

  // The session is really gone, not just hidden: a private page bounces back to login.
  await page.goto("/today");
  await expect(page).toHaveURL(/\/login\?next=%2Ftoday$/);
});

// The API allows 10 auth requests a minute per IP, so the flows below run on the desktop project only.
test("a private page asked for while signed out is reached after login", async ({ page, isMobile }) => {
  test.skip(isMobile, "desktop project only (auth rate limit)");
  const email = freshEmail();
  await register(page, email);
  await page.context().clearCookies();

  await page.goto("/review");
  await expect(page).toHaveURL(/\/login\?next=%2Freview$/);
  await page.getByLabel("Email", { exact: true }).fill(email);
  await page.getByLabel("Kata sandi", { exact: true }).fill(PASSWORD);
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page).toHaveURL(/\/review$/);
});

test("a wrong password says so at form level and keeps the person on /login", async ({ page, isMobile }) => {
  test.skip(isMobile, "desktop project only (auth rate limit)");
  await page.goto("/login");
  await page.getByLabel("Email", { exact: true }).fill("nobody@example.test");
  await page.getByLabel("Kata sandi", { exact: true }).fill("salah-sekali-1");
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page.locator("form").getByRole("alert")).toContainText("Email atau kata sandi belum cocok");
  await expect(page).toHaveURL(/\/login$/);
});

test("the register form catches a short password before asking the server", async ({ page }) => {
  await page.goto("/register");
  await page.getByLabel("Nama", { exact: true }).fill("Sari");
  await page.getByLabel("Email", { exact: true }).fill(freshEmail());
  await page.getByLabel("Kata sandi", { exact: true }).fill("pendek");
  await page.getByRole("button", { name: "Buat akun" }).click();
  await expect(page.locator("form").getByRole("alert")).toContainText("minimal 8 karakter");
  await expect(page.getByLabel("Kata sandi", { exact: true })).toBeFocused();
});
