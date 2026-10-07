import { expect, test } from "bun:test";
import { supervise } from "./supervisor.ts";

const bun = process.execPath;

test("prefixes each line with the process name", async () => {
  const lines: string[] = [];
  let release!: () => void;
  const stop = new Promise<void>((r) => (release = r));
  const run = supervise(
    [{ name: "echo", cmd: [bun, "-e", "console.log('one'); console.log('two'); setInterval(() => {}, 1000)"] }],
    { out: (l) => lines.push(l), stopSignal: stop },
  );
  await Bun.sleep(800);
  release();
  expect(await run).toBe(0);
  const plain = lines.map((l) => l.replace(/\x1b\[[0-9;]*m/g, ""));
  expect(plain).toContain("echo │ one");
  expect(plain).toContain("echo │ two");
});

test("a crashing child stops the others and returns its exit code", async () => {
  const lines: string[] = [];
  const started = Date.now();
  const code = await supervise(
    [
      { name: "long", cmd: [bun, "-e", "setInterval(() => {}, 1000)"] },
      { name: "bad", cmd: [bun, "-e", "setTimeout(() => process.exit(3), 200)"] },
    ],
    { out: (l) => lines.push(l), stopSignal: new Promise(() => {}) },
  );
  expect(code).toBe(3);
  expect(Date.now() - started).toBeLessThan(5000);
  expect(lines.join("\n")).toContain("bad crashed (exit 3)");
});
