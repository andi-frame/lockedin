import { expect, test } from "bun:test";
import { composeArgs, envFilePath, parseScaleArgs, parseUpArgs, problemsInEnv, profilesFor, projectName, scaleCommand } from "./deploy.ts";
import { parseEnv } from "./env.ts";

const good = `
APP_BASE_URL=https://staging.example.org
DOMAIN=staging.example.org
SESSION_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
POSTGRES_PASSWORD=pw
DATABASE_URL=postgres://tepati:\${POSTGRES_PASSWORD}@postgres:5432/tepati?sslmode=disable
REDIS_URL=redis://redis:6379/0
S3_ENDPOINT=http://garage:3900
S3_PUBLIC_ENDPOINT=https://media.staging.example.org
GARAGE_RPC_SECRET=a
GARAGE_ADMIN_TOKEN=b
GARAGE_METRICS_TOKEN=c
SMTP_URL=smtp://mailpit:1025
MAIL_FROM="Tepati <no-reply@staging.example.org>"
`;

test("each environment is its own compose project, so staging never shares volumes with dev", () => {
  expect(projectName("staging")).toBe("tepati-staging");
  expect(projectName("production")).toBe("tepati-production");
});

test("compose gets both files and the environment's env file, from the repo root's deploy folder", () => {
  expect(envFilePath("staging", "/repo")).toBe("/repo/deploy/env/.env.staging");
  expect(composeArgs("production", "/repo")).toEqual([
    "docker", "compose", "-p", "tepati-production",
    "-f", "/repo/deploy/compose.yaml", "-f", "/repo/deploy/compose.prod.yaml",
    "--env-file", "/repo/deploy/env/.env.production",
  ]);
});

test("a complete env file has no problems", () => {
  expect(problemsInEnv("staging", parseEnv(good))).toEqual([]);
});

test("a missing key and a leftover CHANGE_ME are both named", () => {
  const env = parseEnv(good.replace("SESSION_SECRET=0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", "SESSION_SECRET=CHANGE_ME").replace(/^REDIS_URL=.*$/m, ""));
  const problems = problemsInEnv("staging", env);
  expect(problems).toContain("SESSION_SECRET still says CHANGE_ME");
  expect(problems).toContain("REDIS_URL is missing");
});

test("an empty S3 key is allowed (the first deploy generates it) but an empty password is not", () => {
  const env = parseEnv(good + "S3_ACCESS_KEY=\nS3_SECRET_KEY=\n");
  expect(problemsInEnv("staging", env)).toEqual([]);
  expect(problemsInEnv("staging", parseEnv(good.replace("POSTGRES_PASSWORD=pw", "POSTGRES_PASSWORD=")))).toContain("POSTGRES_PASSWORD is missing");
});

test("production must be served over https; staging may be local http only through https too", () => {
  const prod = parseEnv(good.replace("https://staging.example.org", "http://staging.example.org"));
  expect(problemsInEnv("production", prod)).toContain("APP_BASE_URL must start with https:// in production");
  expect(problemsInEnv("staging", prod)).toEqual([]);
});

test("the S3 public endpoint must be on the media host, not a path prefix (a presigned URL signs its path)", () => {
  const env = parseEnv(good.replace("https://media.staging.example.org", "https://staging.example.org/s3"));
  expect(problemsInEnv("staging", env)).toContain("S3_PUBLIC_ENDPOINT must be https://media.<DOMAIN> (a path prefix breaks presigned URLs)");
});

test("the sandbox mail profile is on only when SMTP_URL points at Mailpit", () => {
  expect(profilesFor(parseEnv(good), { backup: false })).toEqual(["infra", "app", "edge", "mail-sandbox"]);
  const real = parseEnv(good.replace("smtp://mailpit:1025", "smtps://u:p@smtp.example.org:465"));
  expect(profilesFor(real, { backup: false })).toEqual(["infra", "app", "edge"]);
});

test("backups are an extra profile", () => {
  const real = parseEnv(good.replace("smtp://mailpit:1025", "smtps://u:p@smtp.example.org:465"));
  expect(profilesFor(real, { backup: true })).toEqual(["infra", "app", "edge", "backup"]);
});

test("deploy:up takes --env, and backups default to on only in production", () => {
  expect(parseUpArgs(["--env", "staging"])).toEqual({ env: "staging", backup: false });
  expect(parseUpArgs(["--env", "production"])).toEqual({ env: "production", backup: true });
  expect(parseUpArgs(["--env", "production", "--no-backup"])).toEqual({ env: "production", backup: false });
  expect(parseUpArgs(["--env=staging", "--backup"])).toEqual({ env: "staging", backup: true });
  expect(parseUpArgs([])).toEqual({ error: "--env is required (staging or production)" });
  expect(parseUpArgs(["--env", "staging", "--force"])).toEqual({ error: 'unknown argument "--force"' });
});

test("scale takes service=count pairs for the stateless services only", () => {
  expect(parseScaleArgs(["--env", "staging", "api=3", "web=2", "worker=2"])).toEqual({ env: "staging", scale: { api: 3, web: 2, worker: 2 } });
  expect(parseScaleArgs(["api=2"])).toEqual({ error: "--env is required (staging or production)" });
  expect(parseScaleArgs(["--env", "staging"])).toEqual({ error: "give at least one service=count, for example api=2" });
  expect(parseScaleArgs(["--env", "staging", "postgres=2"])).toEqual({ error: 'cannot scale "postgres" (use api, web or worker)' });
  expect(parseScaleArgs(["--env", "staging", "api=0"])).toEqual({ error: '"api=0" needs a count from 1 to 20' });
  expect(parseScaleArgs(["--env", "staging", "api=lots"])).toEqual({ error: '"api=lots" needs a count from 1 to 20' });
  expect(parseScaleArgs(["--env", "staging", "api"])).toEqual({ error: '"api" is not service=count' });
});

test("scaling recreates nothing else: no dependencies, one up for the named services", () => {
  expect(scaleCommand(["docker", "compose"], { api: 3, web: 2 })).toEqual([
    "docker", "compose", "up", "-d", "--no-deps", "--no-recreate", "--scale", "api=3", "--scale", "web=2", "api", "web",
  ]);
});
