import { afterAll, expect, test } from "bun:test";
import { cpSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { parseEnv } from "./lib/env.ts";

const tmp = mkdtempSync(join(tmpdir(), "tepati-setup-"));
afterAll(() => rmSync(tmp, { recursive: true, force: true }));

function runSetup() {
  const proc = Bun.spawnSync(["bun", join(import.meta.dir, "setup.ts")], {
    env: { ...process.env, TEPATI_ROOT: tmp },
  });
  expect(proc.exitCode).toBe(0);
  return proc.stdout.toString();
}

test("setup creates env files once and is idempotent", async () => {
  // Copy only the committed templates, never a developer's real env files.
  cpSync(join(import.meta.dir, "..", "deploy", "env"), join(tmp, "deploy", "env"), {
    recursive: true,
    filter: (src) => !/\.env(\.[a-z]+)?$/.test(src) || src.endsWith(".example"),
  });

  const first = runSetup();
  expect(first).toContain(".env: created");
  expect(first).toContain("deploy/env/.env.dev: created");

  const rootText = await Bun.file(join(tmp, ".env")).text();
  const devText = await Bun.file(join(tmp, "deploy", "env", ".env.dev")).text();
  expect(rootText).not.toMatch(/=__generate/);
  expect(devText).not.toMatch(/=__from_root__/);

  const root = parseEnv(rootText);
  const dev = parseEnv(devText);
  expect(root.get("GARAGE_RPC_SECRET")).toMatch(/^[0-9a-f]{64}$/);
  expect(dev.get("POSTGRES_PASSWORD")).toBe(root.get("POSTGRES_PASSWORD")!);
  expect(dev.get("GARAGE_RPC_SECRET")).toBe(root.get("GARAGE_RPC_SECRET")!);
  expect(root.get("DATABASE_URL")).toContain(`:${root.get("POSTGRES_PASSWORD")}@localhost:55432/`);
  expect(dev.get("DATABASE_URL")).toContain("@postgres:5432/");

  const second = runSetup();
  expect(second).toContain(".env: exists, skipped");
  expect(second).toContain("deploy/env/.env.dev: exists, skipped");
  expect(await Bun.file(join(tmp, ".env")).text()).toBe(rootText);
});
