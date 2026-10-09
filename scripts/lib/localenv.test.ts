import { expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { problemsInEnv } from "./deploy.ts";
import { parseEnv } from "./env.ts";
import { renderLocalStagingEnv } from "./localenv.ts";

const template = readFileSync(new URL("../../deploy/env/.env.staging.example", import.meta.url), "utf8");
let n = 0;
const secret = (kind: string) => `${kind}-${++n}`.padEnd(64, "x");

test("the generated file passes the same check deploy:up applies, with no CHANGE_ME left", () => {
  const out = renderLocalStagingEnv(template, secret);
  expect(out).not.toContain("CHANGE_ME");
  expect(problemsInEnv("staging", parseEnv(out))).toEqual([]);
});

test("it points at localhost, on the media host and https, and keeps Mailpit", () => {
  const env = parseEnv(renderLocalStagingEnv(template, secret));
  expect(env.get("DOMAIN")).toBe("localhost");
  expect(env.get("APP_BASE_URL")).toBe("https://localhost");
  expect(env.get("S3_PUBLIC_ENDPOINT")).toBe("https://media.localhost");
  expect(env.get("SMTP_URL")).toBe("smtp://mailpit:1025");
});

test("every secret is generated, and no two are the same", () => {
  const env = parseEnv(renderLocalStagingEnv(template, secret));
  const keys = ["SESSION_SECRET", "POSTGRES_PASSWORD", "GARAGE_RPC_SECRET", "GARAGE_ADMIN_TOKEN", "GARAGE_METRICS_TOKEN"];
  const values = keys.map((k) => env.get(k));
  expect(values.every(Boolean)).toBe(true);
  expect(new Set(values).size).toBe(keys.length);
});

test("the S3 key pair stays empty: the first deploy:up generates it", () => {
  const env = parseEnv(renderLocalStagingEnv(template, secret));
  expect(env.get("S3_ACCESS_KEY")).toBe("");
  expect(env.get("S3_SECRET_KEY")).toBe("");
});

test("the rate limits are raised for a test suite, in this file only", () => {
  const env = parseEnv(renderLocalStagingEnv(template, secret));
  expect(env.get("AUTH_RATE_LIMIT_PER_MIN")).toBe("200");
  expect(env.get("RATE_LIMIT_PER_MIN")).toBe("3000");
  expect(parseEnv(template).has("AUTH_RATE_LIMIT_PER_MIN")).toBe(false);
});
