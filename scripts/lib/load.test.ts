import { expect, test } from "bun:test";
import { k6Command, parseLoadArgs, rateLimitProblem, usersFromSeeds } from "./load.ts";

test("defaults are the PLAN's: 200 reads and 20 writes a second for five minutes, staging", () => {
  expect(parseLoadArgs(["--env", "staging"])).toEqual({ env: "staging", users: 20, duration: "5m", readRate: 200, writeRate: 20 });
});

test("every number can be changed, in either flag form", () => {
  expect(parseLoadArgs(["--env=production", "--users", "5", "--duration=30s", "--read-rate", "50", "--write-rate=5"])).toEqual({
    env: "production", users: 5, duration: "30s", readRate: 50, writeRate: 5,
  });
});

test("bad values are named", () => {
  expect(parseLoadArgs([])).toEqual({ error: "--env is required (staging or production)" });
  expect(parseLoadArgs(["--env", "staging", "--users", "0"])).toEqual({ error: "--users needs a whole number from 1 to 200" });
  expect(parseLoadArgs(["--env", "staging", "--duration", "five minutes"])).toEqual({ error: '--duration must look like 30s, 5m or 1h (got "five minutes")' });
  expect(parseLoadArgs(["--env", "staging", "--read-rate", "-1"])).toEqual({ error: "--read-rate needs a whole number from 1 to 5000" });
  expect(parseLoadArgs(["--env", "staging", "--nope", "1"])).toEqual({ error: 'unknown argument "--nope"' });
});

test("users come from the seed output, doers first, and a seed with no doer line is an error", () => {
  const a = "pact  1  (active)\nbacker  b1@x.test  password pw1\ndoer  d1@x.test  password pw1\n";
  const b = "pact  2  (active)\nbacker  b2@x.test  password pw2\ndoer  d2@x.test  password pw2\n";
  expect(usersFromSeeds([a, b])).toEqual([
    { email: "d1@x.test", password: "pw1" },
    { email: "d2@x.test", password: "pw2" },
  ]);
  expect(() => usersFromSeeds(["nothing here"])).toThrow("no doer login");
});

test("k6 runs from its image on the deploy's network, with the limits and the address it needs", () => {
  const cmd = k6Command({ network: "tepati-staging_default", caddyIp: "172.18.0.9", dir: "/repo/tests/load", opts: { env: "staging", users: 20, duration: "5m", readRate: 200, writeRate: 20 } });
  expect(cmd.slice(0, 3)).toEqual(["docker", "run", "--rm"]);
  expect(cmd).toContain("tepati-staging_default");
  expect(cmd).toContain("grafana/k6:latest");
  expect(cmd.join(" ")).toContain("-e CADDY_IP=172.18.0.9");
  expect(cmd.join(" ")).toContain("-e READ_RATE=200");
  expect(cmd.join(" ")).toContain("-e WRITE_RATE=20");
  expect(cmd.join(" ")).toContain("-e DURATION=5m");
  expect(cmd.join(" ")).toContain("/repo/tests/load:/load");
  expect(cmd.at(-2)).toBe("run");
  expect(cmd.at(-1)).toBe("/load/today.js");
});

test("the API's per-IP budget must cover the whole test, or the test measures the limiter", () => {
  const opts = { env: "staging", users: 20, duration: "5m", readRate: 200, writeRate: 20 } as const;
  // 220 requests a second is 13,200 a minute from the one k6 address.
  expect(rateLimitProblem(new Map([["RATE_LIMIT_PER_MIN", "3000"]]), opts)).toContain("RATE_LIMIT_PER_MIN=3000");
  expect(rateLimitProblem(new Map(), opts)).toContain("300");
  expect(rateLimitProblem(new Map([["RATE_LIMIT_PER_MIN", "20000"]]), opts)).toBeNull();
});
