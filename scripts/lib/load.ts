// Planning for `bun run load`: the options, the users to log in as, and the k6 command.
import { ENVIRONMENTS, type Environment } from "./images.ts";
import { parseSeedLogin } from "./seed-output.ts";

export interface LoadOptions {
  env: Environment;
  users: number;
  duration: string;
  readRate: number;
  writeRate: number;
}

const FLAGS = ["--env", "--users", "--duration", "--read-rate", "--write-rate"];

function whole(raw: string | undefined, max: number): number | undefined {
  return raw !== undefined && /^\d+$/.test(raw) && Number(raw) >= 1 && Number(raw) <= max ? Number(raw) : undefined;
}

/** The PLAN's load: 200 reads and 20 writes a second for five minutes. */
export function parseLoadArgs(argv: string[]): LoadOptions | { error: string } {
  const values = new Map<string, string>();
  for (let i = 0; i < argv.length; i++) {
    const [flag = "", inline] = (argv[i] ?? "").split(/=(.*)/s, 2);
    if (!FLAGS.includes(flag)) return { error: `unknown argument "${flag}"` };
    const value = inline ?? argv[++i];
    if (value !== undefined) values.set(flag, value);
  }
  const env = values.get("--env");
  if (!env) return { error: "--env is required (staging or production)" };
  if (!(ENVIRONMENTS as readonly string[]).includes(env)) return { error: `unknown environment "${env}" (use staging or production)` };

  const users = values.has("--users") ? whole(values.get("--users"), 200) : 20;
  if (users === undefined) return { error: "--users needs a whole number from 1 to 200" };
  const duration = values.get("--duration") ?? "5m";
  if (!/^\d+[smh]$/.test(duration)) return { error: `--duration must look like 30s, 5m or 1h (got "${duration}")` };
  const readRate = values.has("--read-rate") ? whole(values.get("--read-rate"), 5000) : 200;
  if (readRate === undefined) return { error: "--read-rate needs a whole number from 1 to 5000" };
  const writeRate = values.has("--write-rate") ? whole(values.get("--write-rate"), 5000) : 20;
  if (writeRate === undefined) return { error: "--write-rate needs a whole number from 1 to 5000" };
  return { env: env as Environment, users, duration, readRate, writeRate };
}

/** The doers of several `tepatictl seed` outputs: they are the ones who submit proof and read their Today. */
export function usersFromSeeds(outputs: string[]): { email: string; password: string }[] {
  return outputs.map((out, i) => {
    const doer = parseSeedLogin(out, "doer");
    if (!doer) throw new Error(`seed ${i + 1} printed no doer login`);
    return doer;
  });
}

/**
 * k6 from its image, on the deploy's compose network. `CADDY_IP` lets the script keep calling
 * https://localhost (Caddy only has a certificate and a site for that name) while the packets go
 * to the Caddy container.
 */
export function k6Command(args: { network: string; caddyIp: string; dir: string; opts: LoadOptions }): string[] {
  const { opts } = args;
  return [
    "docker", "run", "--rm", "-i", "--network", args.network,
    "-v", `${args.dir}:/load`,
    "-e", `CADDY_IP=${args.caddyIp}`,
    "-e", `READ_RATE=${opts.readRate}`,
    "-e", `WRITE_RATE=${opts.writeRate}`,
    "-e", `DURATION=${opts.duration}`,
    "-e", "K6_INSECURE_SKIP_TLS_VERIFY=true",
    "grafana/k6:latest",
    "run", "/load/today.js",
  ];
}

/**
 * Every request comes from the one k6 address, and the API limits requests per address. Below the
 * test's own rate (with a margin) the run would measure the limiter, not the application.
 */
export function rateLimitProblem(envValues: Map<string, string>, opts: LoadOptions): string | null {
  const perMinute = (opts.readRate + opts.writeRate) * 60;
  const need = Math.ceil(perMinute * 1.2);
  const have = Number(envValues.get("RATE_LIMIT_PER_MIN") ?? 300);
  if (have >= need) return null;
  return `RATE_LIMIT_PER_MIN=${have} in the env file is below the ${need} this load needs (${perMinute} requests a minute plus a margin); raise it, restart the api, and say so when you report the numbers`;
}
