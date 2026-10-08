import { describe, expect, test } from "bun:test";
import { layerNativeEnv, materialize, pickDatabaseUrl, parseEnv, setEnvValue } from "./env.ts";

describe("parseEnv", () => {
  test("handles comments, quotes, and inline comments", () => {
    const env = parseEnv(
      ["# header", "A=1", "B=two   # note", 'C="Tepati <a@b> # not a comment"', "export D=x", ""].join("\n"),
    );
    expect(env.get("A")).toBe("1");
    expect(env.get("B")).toBe("two");
    expect(env.get("C")).toBe("Tepati <a@b> # not a comment");
    expect(env.get("D")).toBe("x");
  });
});

describe("setEnvValue", () => {
  test("replaces a value and keeps the inline comment", () => {
    const out = setEnvValue("S3_ACCESS_KEY=        # written later\nX=1\n", "S3_ACCESS_KEY", "GK123");
    expect(parseEnv(out).get("S3_ACCESS_KEY")).toBe("GK123");
    expect(out).toContain("# written later");
    expect(parseEnv(out).get("X")).toBe("1");
  });
  test("appends a missing key", () => {
    expect(parseEnv(setEnvValue("A=1\n", "B", "2")).get("B")).toBe("2");
  });
});

describe("materialize", () => {
  test("generates secrets, copies root values, and expands references", () => {
    const template = [
      "SECRET=__generate:hex32__",
      "PW=__from_root__",
      "URL=postgres://u:${PW}@h/db",
    ].join("\n");
    const env = parseEnv(materialize(template, { fromRoot: new Map([["PW", "pass"]]) }));
    expect(env.get("SECRET")).toMatch(/^[0-9a-f]{64}$/);
    expect(env.get("PW")).toBe("pass");
    expect(env.get("URL")).toBe("postgres://u:pass@h/db");
  });
  test("fails loudly on an undefined reference", () => {
    expect(() => materialize("URL=${NOPE}")).toThrow(/NOPE/);
  });
});

test("native mode layers .env.native over .env and always uses the fs driver (Garage has no Windows build)", () => {
  const base = new Map([["DATABASE_URL", "postgres://docker"], ["STORAGE_DRIVER", "s3"], ["API_PORT", "18080"]]);
  const native = new Map([["DATABASE_URL", "postgres://native"], ["REDIS_URL", "redis://native"]]);
  const out = layerNativeEnv(base, native);
  expect(out.get("DATABASE_URL")).toBe("postgres://native");
  expect(out.get("REDIS_URL")).toBe("redis://native");
  expect(out.get("API_PORT")).toBe("18080");
  expect(out.get("STORAGE_DRIVER")).toBe("fs");
});

test("layering does not change its inputs", () => {
  const base = new Map([["A", "1"]]);
  layerNativeEnv(base, new Map([["A", "2"]]));
  expect(base.get("A")).toBe("1");
});

// Bun loads the root `.env` into process.env by itself, so a DATABASE_URL in the environment is
// usually the Docker one; in native mode it must not beat `.env.native`.
describe("pickDatabaseUrl", () => {
  const files = new Map([["DATABASE_URL", "postgres://from-file"]]);
  test("an explicit environment variable wins outside native mode", () => {
    expect(pickDatabaseUrl({ DATABASE_URL: "postgres://explicit" }, files, false)).toBe("postgres://explicit");
    expect(pickDatabaseUrl({}, files, false)).toBe("postgres://from-file");
  });
  test("in native mode the layered file wins over the environment", () => {
    expect(pickDatabaseUrl({ DATABASE_URL: "postgres://docker-from-dotenv" }, files, true)).toBe("postgres://from-file");
  });
  test("nothing set is undefined", () => {
    expect(pickDatabaseUrl({}, new Map(), false)).toBeUndefined();
  });
});
