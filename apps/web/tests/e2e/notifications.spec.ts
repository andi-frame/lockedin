import { expect, test, type Page } from "@playwright/test";

// `tepatictl seed --scenario review` leaves notifications behind through the real service: the
// backer was told about three proofs (proof_submitted), and the doer that one of them was approved
// automatically (proof_auto_approved). The worker's outbox relay turns those into inbox rows, so
// the first assertions wait for them instead of assuming they are already there.
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

const unreadBell = (page: Page) => page.getByRole("link", { name: /^Notifikasi, \d+ belum dibaca$/ });
const quietBell = (page: Page) => page.getByRole("link", { name: "Notifikasi", exact: true });

test("the bell opens the inbox, which reads each notification, links to where it points and clears the count", async ({ page, isMobile }) => {
  test.setTimeout(Math.max(120_000, Number(process.env.E2E_TIMEOUT_MS ?? 0)));
  await signIn(page, emails(isMobile).backer);

  // The relay is asynchronous: reload Today until the bell has a count.
  await expect(async () => {
    await page.goto("/today");
    await expect(unreadBell(page)).toBeVisible({ timeout: 2000 });
  }).toPass({ timeout: 60_000 });

  const marked = page.waitForResponse((r) => r.url().includes("/notifications/read") && r.request().method() === "POST");
  await unreadBell(page).click();
  await expect(page).toHaveURL(/\/notifications$/);
  await expect(page.getByRole("heading", { name: "Notifikasi", level: 1 })).toBeVisible();

  const list = page.getByTestId("notifications");
  // A pact leaves several rows (scheduled, signed, proof sent), so a row is picked by its kind too.
  const approve = list.locator('li[data-kind="proof_submitted"]', { hasText: "Review: approve" });
  await expect(approve).toHaveCount(1);
  await expect(approve.getByText("Ada bukti baru yang menunggu tinjauanmu.")).toBeVisible();
  await expect(approve.getByText("Baru", { exact: true })).toBeVisible(); // not colour alone

  // Opening the inbox marks what was on the page as read, and the "Baru" marks stay for this visit.
  expect((await marked).status()).toBe(204);
  await expect(approve.getByText("Baru", { exact: true })).toBeVisible();

  // The bell (on Today, where a phone has it) lost its count, and the next visit shows them as read.
  await page.goto("/today");
  await expect(quietBell(page)).toBeVisible();
  await expect(unreadBell(page)).toHaveCount(0);
  await page.goto("/notifications");
  await expect(list.locator('li[data-kind="proof_submitted"]', { hasText: "Review: approve" })).toHaveAttribute("data-unread", "false");
  await expect(list.getByText("Baru", { exact: true })).toHaveCount(0);

  // A check-in notification opens that day, for the member who owes it.
  await list.locator('li[data-kind="proof_submitted"]', { hasText: "Review: approve" }).getByRole("link").click();
  await expect(page).toHaveURL(/\/pacts\/[0-9a-f-]{36}\/days\/\d{4}-\d{2}-\d{2}\?of=[0-9a-f-]{36}$/);
});

test("the doer is told, in words, that a proof was approved automatically", async ({ page, isMobile }) => {
  test.setTimeout(Math.max(120_000, Number(process.env.E2E_TIMEOUT_MS ?? 0)));
  await signIn(page, emails(isMobile).doer);

  const row = page.getByTestId("notifications").locator('li[data-kind="proof_auto_approved"]', { hasText: "Review: override" });
  await expect(async () => {
    await page.goto("/notifications");
    await expect(row).toHaveCount(1, { timeout: 2000 });
  }).toPass({ timeout: 60_000 });
  await expect(row.getByText("Buktimu disetujui otomatis karena tinjauan tidak selesai tepat waktu.")).toBeVisible();

  await row.getByRole("link").click();
  await expect(page).toHaveURL(/\/pacts\/[0-9a-f-]{36}\/days\/\d{4}-\d{2}-\d{2}\?of=[0-9a-f-]{36}$/);
});

test("settings point to the inbox and say what cannot be changed yet", async ({ page, isMobile }) => {
  await signIn(page, emails(isMobile).backer);
  await page.goto("/settings");
  await expect(page.getByText("Nama, zona waktu, dan bahasa belum bisa diubah dari aplikasi.")).toBeVisible();
  await page.getByRole("link", { name: "Buka notifikasi" }).click();
  await expect(page).toHaveURL(/\/notifications$/);
});
