// Planning for `bun run deploy:up` and `deploy:scale`: which compose project, files and profiles an
// environment uses, and whether its env file is fit to deploy. Pure, so it is tested without Docker.
import type { EnvMap } from "./env.ts";
import { ENVIRONMENTS, type Environment } from "./images.ts";

export type { Environment };
export const SCALABLE = ["api", "web", "worker"] as const;
export type Scalable = (typeof SCALABLE)[number];

/** One compose project per environment: staging and production never share containers or volumes with dev. */
export const projectName = (env: Environment) => `tepati-${env}`;
export const envFilePath = (env: Environment, root: string) => `${root}/deploy/env/.env.${env}`;

export function composeArgs(env: Environment, root: string): string[] {
  return [
    "docker", "compose", "-p", projectName(env),
    "-f", `${root}/deploy/compose.yaml`, "-f", `${root}/deploy/compose.prod.yaml`,
    "--env-file", envFilePath(env, root),
  ];
}

const REQUIRED = [
  "APP_BASE_URL", "DOMAIN", "SESSION_SECRET", "POSTGRES_PASSWORD", "DATABASE_URL", "REDIS_URL", "S3_ENDPOINT",
  "S3_PUBLIC_ENDPOINT", "GARAGE_RPC_SECRET", "GARAGE_ADMIN_TOKEN", "GARAGE_METRICS_TOKEN", "SMTP_URL", "MAIL_FROM",
];

/** What is wrong with an env file, one sentence each; an empty list means it can be deployed. */
export function problemsInEnv(env: Environment, values: EnvMap): string[] {
  const problems: string[] = [];
  for (const key of REQUIRED) {
    if (!values.get(key)) problems.push(`${key} is missing`);
  }
  // Any value left from the template, empty or not (the S3 keys may be empty: the first deploy generates them).
  for (const [key, value] of values) {
    if (value.includes("CHANGE_ME")) problems.push(`${key} still says CHANGE_ME`);
  }
  const base = values.get("APP_BASE_URL");
  if (env === "production" && base && !base.startsWith("https://")) problems.push("APP_BASE_URL must start with https:// in production");
  const domain = values.get("DOMAIN");
  const pub = values.get("S3_PUBLIC_ENDPOINT");
  if (domain && pub && !pub.startsWith(`https://media.${domain}`)) {
    problems.push("S3_PUBLIC_ENDPOINT must be https://media.<DOMAIN> (a path prefix breaks presigned URLs)");
  }
  return problems;
}

/** Staging keeps Mailpit so no real mail leaves; its profile runs only while SMTP_URL points at it. */
export function profilesFor(values: EnvMap, opts: { backup: boolean }): string[] {
  const profiles = ["infra", "app", "edge"];
  let host = "";
  try {
    host = new URL(values.get("SMTP_URL") ?? "").hostname;
  } catch {
    // an unparsable SMTP_URL is reported by problemsInEnv's callers through the API's own config check
  }
  if (host === "mailpit") profiles.push("mail-sandbox");
  if (opts.backup) profiles.push("backup");
  return profiles;
}

const ENV_REQUIRED = "--env is required (staging or production)";
const isEnv = (v: string): v is Environment => (ENVIRONMENTS as readonly string[]).includes(v);

/** Splits `--flag value` and `--flag=value`; returns the flags and the leftover positional arguments. */
function readFlags(argv: string[], valued: string[], bare: string[]): { flags: Map<string, string | true>; rest: string[] } | { error: string } {
  const flags = new Map<string, string | true>();
  const rest: string[] = [];
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i] ?? "";
    if (!arg.startsWith("--")) {
      rest.push(arg);
      continue;
    }
    const [flag = "", inline] = arg.split(/=(.*)/s, 2);
    if (bare.includes(flag)) flags.set(flag, true);
    else if (valued.includes(flag)) {
      const value = inline ?? (argv[i + 1]?.startsWith("--") ? undefined : argv[++i]);
      if (value === undefined) return { error: ENV_REQUIRED };
      flags.set(flag, value);
    } else return { error: `unknown argument "${flag}"` };
  }
  return { flags, rest };
}

export function parseUpArgs(argv: string[]): { env: Environment; backup: boolean } | { error: string } {
  const read = readFlags(argv, ["--env"], ["--backup", "--no-backup"]);
  if ("error" in read) return read;
  const env = read.flags.get("--env");
  if (typeof env !== "string") return { error: ENV_REQUIRED };
  if (!isEnv(env)) return { error: `unknown environment "${env}" (use staging or production)` };
  const backup = read.flags.has("--no-backup") ? false : read.flags.has("--backup") ? true : env === "production";
  return { env, backup };
}

export function parseScaleArgs(argv: string[]): { env: Environment; scale: Partial<Record<Scalable, number>> } | { error: string } {
  const read = readFlags(argv, ["--env"], []);
  if ("error" in read) return read;
  const env = read.flags.get("--env");
  if (typeof env !== "string") return { error: ENV_REQUIRED };
  if (!isEnv(env)) return { error: `unknown environment "${env}" (use staging or production)` };
  if (read.rest.length === 0) return { error: "give at least one service=count, for example api=2" };
  const scale: Partial<Record<Scalable, number>> = {};
  for (const pair of read.rest) {
    const [name, count, ...extra] = pair.split("=");
    if (count === undefined || extra.length) return { error: `"${pair}" is not service=count` };
    if (!(SCALABLE as readonly string[]).includes(name ?? "")) return { error: `cannot scale "${name}" (use api, web or worker)` };
    const n = Number(count);
    if (!/^\d+$/.test(count) || n < 1 || n > 20) return { error: `"${pair}" needs a count from 1 to 20` };
    scale[name as Scalable] = n;
  }
  return { env, scale };
}

/** One `up` for the named services: no dependencies touched, nothing already running recreated. */
export function scaleCommand(compose: string[], scale: Partial<Record<Scalable, number>>): string[] {
  const entries = Object.entries(scale);
  return [
    ...compose, "up", "-d", "--no-deps", "--no-recreate",
    ...entries.flatMap(([name, n]) => ["--scale", `${name}=${n}`]),
    ...entries.map(([name]) => name),
  ];
}
