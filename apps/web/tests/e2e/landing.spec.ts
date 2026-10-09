import { expect, test } from "@playwright/test";

const freshEmail = () => `e2e-${Date.now()}-${Math.random().toString(36).slice(2, 8)}@example.test`;

test("a visitor sees the landing page at / and its buttons lead to register and login", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole("heading", { level: 1, name: "Janji belajar yang dijaga berdua." })).toBeVisible();
  // The money honesty line is in the first screen, not in the footer.
  await expect(page.getByText("Tidak ada uang yang lewat Tepati", { exact: false })).toBeInViewport();
  // The example is labelled as made-up data.
  await expect(page.getByText("Data contoh, bukan pengguna sungguhan", { exact: false })).toBeVisible();

  await page.getByRole("link", { name: "Buat akun" }).first().click();
  await expect(page).toHaveURL(/\/register$/);
  await page.goto("/");
  await page.getByRole("link", { name: "Masuk" }).first().click();
  await expect(page).toHaveURL(/\/login$/);
});

test("the landing page has no horizontal scroll and one h1", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toHaveCount(1);
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});

test("a signed-in visitor at / goes straight to /today", async ({ page }) => {
  await page.goto("/register");
  await page.getByLabel("Nama", { exact: true }).fill("Sari E2E");
  await page.getByLabel("Email", { exact: true }).fill(freshEmail());
  await page.getByLabel("Kata sandi", { exact: true }).fill("kata-sandi-e2e-1");
  await page.getByRole("button", { name: "Buat akun" }).click();
  await expect(page).toHaveURL(/\/today$/);
  await page.goto("/");
  await expect(page).toHaveURL(/\/today$/);
});
