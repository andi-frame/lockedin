import { expect, test, type Page } from "@playwright/test";

// `bun run test:e2e` seeds one doer (tepatictl seed --scenario today) whose four active pacts have
// a check-in dated today that is open, submitted, approved and missed. The states are produced by
// the service rules with a clock, and the missed one by the real sweep, so nothing here fakes a
// status. The doer's own login costs one auth request a minute per project.
const password = process.env.E2E_TODAY_PASSWORD;
const emailFor = (isMobile: boolean) => (isMobile ? process.env.E2E_TODAY_EMAIL_MOBILE : process.env.E2E_TODAY_EMAIL_DESKTOP);

async function signIn(page: Page, isMobile: boolean) {
  await page.goto("/login");
  await page.getByLabel("Email", { exact: true }).fill(emailFor(isMobile) ?? "");
  await page.getByLabel("Kata sandi", { exact: true }).fill(password ?? "");
  await page.getByRole("button", { name: "Masuk" }).click();
  await expect(page).toHaveURL(/\/today$/);
  await expect(page.getByRole("heading", { level: 1, name: "Hari ini" })).toBeVisible();
}

// The "Hari ini" section of one pact (its heading is a link to the pact).
const section = (page: Page, title: string) => page.locator("section").filter({ has: page.getByRole("link", { name: title, exact: true }) });

test.beforeEach(({ isMobile }) => {
  test.skip(!emailFor(isMobile) || !password, "run through `bun run test:e2e`, which seeds the today scenario");
});

test("shows every state of today's check-in, each pact with its own deadline and passbook", async ({ page, isMobile }) => {
  await signIn(page, isMobile);

  // The band counts down to the soonest check-in still waiting to be sent, in fixed cells.
  await expect(page.getByText("Batas terdekat: Today: open")).toBeVisible();
  const band = page.getByRole("main").locator("header").first();
  await expect(band.getByText("lagi", { exact: true })).toBeVisible();
  if (isMobile) await expect(band.getByRole("img", { name: /Notifikasi/ })).toBeVisible();

  // Sections follow the nearest deadline: the missed day's pact first, the open one next.
  const titles = await page.locator("h2 a").allTextContents();
  expect(titles.slice(0, 2)).toEqual(["Today: missed", "Today: open"]);
  expect(titles).toHaveLength(4);

  const expected: [string, string][] = [
    ["Today: missed", "Terlewat"],
    ["Today: open", "Terbuka"],
    ["Today: approved", "Disetujui"],
    ["Today: submitted", "Menunggu tinjauan"],
  ];
  for (const [title, chip] of expected) {
    await expect(section(page, title).getByText(chip, { exact: true })).toBeVisible();
    await expect(section(page, title).getByText("Latihan soal UTBK 50 soal")).toBeVisible();
  }

  // Open: a countdown, the cutoff with its grace, and sending proof says it is not here yet.
  const open = section(page, "Today: open");
  await expect(open.getByText("Batas 23:59, masa tenggang sampai 00:29")).toBeVisible();
  await expect(open.getByRole("button", { name: "Kirim bukti" })).toBeDisabled();
  // Submitted: who is reviewing and by when, with the word count of the proof.
  await expect(section(page, "Today: submitted").getByText(/Menunggu tinjauan Andi \(today\)\. Kalau belum ditinjau sebelum/)).toBeVisible();
  await expect(section(page, "Today: submitted").getByText("5 kata")).toBeVisible();

  // The missed day is one printed debit line, and the saldo is the sum of the lines.
  const missedBook = page.getByRole("table", { name: /Today: missed/ });
  await expect(missedBook.getByRole("row").filter({ hasText: "Terlewat" })).toContainText("50");
  await expect(missedBook.getByRole("row").filter({ hasText: "Terlewat" })).toContainText("950");
  await expect(missedBook.getByRole("row").filter({ hasText: "Pot awal" })).toContainText("1.000");
  await expect(page.getByRole("table", { name: /Today: open/ }).getByRole("row").filter({ hasText: "Pot awal" })).toBeVisible();

  // The backer's proof in the submitted pact is waiting for this doer to review.
  await expect(page.getByRole("heading", { name: "Perlu ditinjau (1)" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Lihat semua" })).toHaveAttribute("href", "/review");
});

test("a rest day is confirmed first, then the check-in shows as rested and the button is gone", async ({ page, isMobile }) => {
  await signIn(page, isMobile);
  const open = section(page, "Today: open");
  await open.getByRole("button", { name: "Ambil hari jeda" }).click();

  const dialog = page.getByRole("dialog", { name: "Ambil hari jeda hari ini?" });
  await expect(dialog).toContainText("dari 2 jadi 1");
  await dialog.getByRole("button", { name: "Batal" }).click();
  await expect(open.getByText("Terbuka", { exact: true })).toBeVisible(); // cancelling changed nothing

  await open.getByRole("button", { name: "Ambil hari jeda" }).click();
  await page.getByRole("dialog").getByRole("button", { name: "Ya, ambil hari jeda" }).click();
  await expect(page.getByText("Hari jeda diambil.", { exact: true })).toBeVisible();
  await expect(open.getByText("Hari jeda", { exact: true })).toBeVisible();
  await expect(open.getByText("Hari jeda: hari ini tidak dihitung.")).toBeVisible();
  await expect(open.getByRole("button", { name: "Ambil hari jeda" })).toHaveCount(0);
  // With nothing left to send, the band no longer counts down to that pact.
  await expect(page.getByText("Batas terdekat: Today: open")).toHaveCount(0);
});
