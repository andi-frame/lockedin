// `bun run load -- --env staging [--users 20] [--duration 5m] [--read-rate 200] [--write-rate 20]`:
// the k6 load test of tests/load/today.js against a running deploy (`bun run deploy:up`). It seeds
// the users through the deploy's api container, writes tests/load/.users.json, and runs k6 from
// its image on the deploy's network. The exit code is k6's: non-zero when a threshold is crossed.
import { join } from "node:path";
import { projectName, envFilePath } from "./lib/deploy.ts";
import { dockerAvailable } from "./lib/compose.ts";
import { readEnvFile } from "./lib/env.ts";
import { k6Command, parseLoadArgs, rateLimitProblem, usersFromSeeds } from "./lib/load.ts";
import { capture, fail, log } from "./lib/proc.ts";
import { paths } from "./lib/paths.ts";

const opts = parseLoadArgs(process.argv.slice(2));
if ("error" in opts) fail(opts.error);
if (!(await dockerAvailable())) fail("Docker is not running.");

const problem = rateLimitProblem(await readEnvFile(envFilePath(opts.env, paths.root)), opts);
if (problem) fail(problem);

const project = projectName(opts.env);
const network = `${project}_default`;
const ip = await capture(["docker", "inspect", "-f", `{{(index .NetworkSettings.Networks "${network}").IPAddress}}`, `${project}-caddy-1`]);
const caddyIp = ip.stdout.trim();
if (ip.code !== 0 || !caddyIp) fail(`no running ${project}-caddy-1: start the deploy first (bun run deploy:up -- --env ${opts.env})`);

log.step(`seeding ${opts.users} users in ${project}`);
const outputs: string[] = [];
for (let i = 0; i < opts.users; i++) {
  const seeded = await capture(["bun", "--no-env-file", join(paths.root, "scripts", "ctl.ts"), "seed", "--scenario", "today"], { env: { TEPATI_CTL_PROJECT: opts.env } });
  if (seeded.code !== 0) fail(`seed ${i + 1} failed:\n${seeded.stderr || seeded.stdout}`);
  outputs.push(seeded.stdout);
}
await Bun.write(join(paths.root, "tests", "load", ".users.json"), JSON.stringify(usersFromSeeds(outputs)));

log.step(`k6: ${opts.readRate} reads/s and ${opts.writeRate} writes/s for ${opts.duration}, ${opts.users} users`);
const cmd = k6Command({ network, caddyIp, dir: join(paths.root, "tests", "load").replaceAll("\\", "/"), opts });
const proc = Bun.spawn(cmd, { stdio: ["inherit", "inherit", "inherit"], env: { ...process.env, MSYS_NO_PATHCONV: "1" } });
process.exit(await proc.exited);
