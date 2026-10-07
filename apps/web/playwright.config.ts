import { defineConfig, devices } from "@playwright/test";

// These tests drive the real stack (web, API, Postgres, Redis): start it first with
// `bun run dev:hybrid`, then run `bun run test:e2e` from the repo root. The root script checks the
// stack is up. The API allows 10 auth requests a minute per IP (ARCHITECTURE §3), so the specs stay
// well under that and run one at a time.
export default defineConfig({
  testDir: "./tests/e2e",
  workers: 1,
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  reporter: [["list"]],
  outputDir: "./test-results",
  use: {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    trace: "retain-on-failure",
  },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
    { name: "mobile", use: { ...devices["Pixel 7"], viewport: { width: 390, height: 844 } } },
  ],
});
