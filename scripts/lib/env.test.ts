import { describe, expect, test } from "bun:test";
import { materialize, parseEnv, setEnvValue } from "./env.ts";

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
