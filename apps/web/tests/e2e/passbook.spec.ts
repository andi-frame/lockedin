import { expect, test, type Page } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

// Signs in as a doer whose pact has 24 finished days behind it (`tepatictl seed --scenario
// passbook`): 33 printed lines, a calendar with every kind of day, and today still open. The miss
// that gets printed in the last two tests is made by `tepatictl advance`, which moves a clock past
// the pact's next deadline and runs the real sweep, so the line comes from the same service code
// the worker uses (decided with the owner on 2026-10-08: no clock endpoint). Each project advances
// its own pact once, to today's cutoff (23:59, no grace), which comes before every other seeded
// deadline, so the sweep it runs touches nothing the other specs rely on.
const password = process.env.E2E_TODAY_PASSWORD;
const emailFor = (isMobile: boolean) => (isMobile ? process.env.E2E_BOOK_EMAIL_MOBILE : process.env.E2E_BOOK_EMAIL_DESKTOP);
const ctl = fileURLToPath(new URL("../../../../scripts/ctl.ts", import.meta.url));

test.beforeEach(({ isMobile }) => {
  test.skip(!emailFor(isMobile) || !password, "run through `bun run test:e2e`, which seeds the passbook scenario");
});

async function openBook(page: Page, isMobile: boolean) {
  await page.goto("/login");
  await page.getByLabel("Email", { exact: true }).fill(emailFor(isMobile) ?? "");
  await page.getByLabel("Kata sandi", { exact: true }).fill(password ?? "");
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page).toHaveURL(/\/today$/);
  await page.getByRole("link", { name: "Passbook: history", exact: true }).first().click();
  await expect(page).toHaveURL(/\/pacts\/[0-9a-f-]{36}$/);
  await expect(page.getByRole("heading", { name: "Passbook: history", level: 1 })).toBeVisible();
  return /\/pacts\/([0-9a-f-]{36})$/.exec(page.url())?.[1] ?? "";
}

const rows = (page: Page) => page.getByTestId("passbook").locator("tbody tr");

function advance(pactId: string) {
  execFileSync("bun", [ctl, "advance", "--pact", pactId], { stdio: "pipe", timeout: 120_000 });
}

test("the coin book shows the saldo, pages in by cursor, and the calendar marks every day", async ({ page, isMobile }) => {
  await openBook(page, isMobile);

  // 1000 - 14 misses x 50 + 18 backer misses x 20 = 660, and the newest line says so too.
  await expect(page.getByText("660").first()).toBeVisible();
  await expect(rows(page)).toHaveCount(20);
  await expect(rows(page).first().locator("td").last()).toHaveText("660");
  await expect(rows(page).first()).toContainText("Terlewat");

  // The rest of the book loads when its end is reached (or by the button).
  await rows(page).last().scrollIntoViewIfNeeded();
  await expect(rows(page)).toHaveCount(33, { timeout: 15_000 });
  await expect(page.getByText("Itu baris pertama buku ini.")).toBeVisible();
  await expect(rows(page).last()).toContainText("Pot awal");
  await expect(rows(page).last().locator("td").last()).toHaveText("1.000");

  // Debits and credits sit in their own columns, each with a sign.
  const miss = page.locator('tr[data-kind="doer_miss"]').first();
  await expect(miss.locator("td").nth(2)).toHaveText("−50");
  await expect(page.locator('tr[data-kind="backer_miss"]').first().locator("td").nth(3)).toHaveText("+20");

  // Today wears the highlighter and carries an open mark for the doer and a sent one for the backer.
  const today = page.getByTestId("calendar").locator("td[data-today]");
  await expect(today).toHaveCount(1);
  await expect(today.locator('[data-status="open"]')).toHaveCount(1);
  await expect(today.locator('[data-status="submitted"]')).toHaveCount(1);

  // Earlier days: approved, rest, rejected and missed all appear somewhere in the pact's two months.
  const calendar = page.getByTestId("calendar");
  const marks = () => calendar.locator("[data-status]").evaluateAll((els) => els.map((e) => e.getAttribute("data-status") ?? ""));
  const seen = new Set(await marks());
  await page.getByRole("button", { name: "Bulan sebelumnya" }).click();
  await expect(page.getByRole("button", { name: "Bulan sebelumnya" })).toBeDisabled(); // the pact starts in September
  for (const s of await marks()) seen.add(s);
  for (const status of ["approved", "rest", "rejected", "missed"]) expect(seen, status).toContain(status);

  // The legend names every symbol in use; colour is never the only signal.
  await expect(page.getByRole("list", { name: "Arti simbol" }).getByText("Terlewat")).toBeVisible();
});

test("a missed day prints a new debit line and the saldo rolls to the new value", async ({ page, isMobile }) => {
  test.skip(isMobile, "the desktop project covers the print motion; mobile covers reduced motion below");
  test.setTimeout(150_000);
  // Fake timers from the start, so the 30-second refresh can be jumped to instead of waited for.
  await page.clock.install();
  const pactId = await openBook(page, isMobile);
  await expect(rows(page)).toHaveCount(20);
  const before = await rows(page).first().getAttribute("data-line-id");

  advance(pactId);
  await page.clock.fastForward(31_000);

  const printed = rows(page).first();
  await expect(printed).not.toHaveAttribute("data-line-id", before ?? "");
  await expect(printed).toHaveAttribute("data-kind", "doer_miss");
  await expect(printed).toHaveClass(/print-line/);
  await expect(printed.locator("td").nth(2)).toHaveText("−50");
  await expect(printed.locator("td").last()).toHaveText("610");
  await expect(rows(page)).toHaveCount(21);
  // The big saldo lands on the server's figure, and today's mark for the doer turned into a miss.
  await expect(page.getByText("610", { exact: true }).first()).toBeVisible({ timeout: 10_000 });
  await expect(page.getByTestId("calendar").locator("td[data-today] [data-status='missed']")).toHaveCount(1);
  // A screen reader hears the new line.
  await expect(page.getByRole("status").filter({ hasText: "Baris baru tercetak" })).toContainText("610");
});

test("with reduced motion the new line is there at once", async ({ page, isMobile }) => {
  test.skip(!isMobile, "one advance per project: desktop spends its pact on the test above");
  test.setTimeout(150_000);
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.clock.install();
  const pactId = await openBook(page, isMobile);
  await expect(rows(page).first()).toBeVisible();
  const before = await rows(page).count();

  advance(pactId);
  await page.clock.fastForward(31_000);

  await expect(rows(page)).toHaveCount(before + 1);
  await expect(rows(page).first()).toHaveAttribute("data-kind", "doer_miss");
  const duration = await rows(page).first().evaluate((el) => getComputedStyle(el).animationDuration);
  expect(duration).toMatch(/^(0s|1e-05s|0\.00001s)$/);
});
