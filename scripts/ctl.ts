// `bun run ctl -- <tepatictl args>` and `bun run db:seed [-- overdue|invite]`:
// the admin CLI (apps/server/cmd/tepatictl) with .env loaded.
//   bun run ctl -- pact show <id>
//   bun run db:seed                      overdue scenario: an active pact the worker should settle
//   bun run db:seed -- invite            a proposed pact with an open invite link
//   bun run db:seed -- today             new users with five active pacts: open, rules, submitted, approved, missed today
//   bun run ctl -- seed --scenario passbook   new users and a pact with 24 days of printed lines
//   bun run ctl -- advance --pact <id>   move a clock past the pact's next deadline and run the real sweep
// With TEPATI_CTL_PROJECT=staging (or production) the same command runs inside that deploy's api
// container (`docker compose exec api tepatictl ...`), against its database; that is how
// `bun run test:e2e` seeds a stack started with `bun run deploy:up`.
import { join } from "node:path";
import { composeArgs, parseUpArgs } from "./lib/deploy.ts";
import { readEnvFile } from "./lib/env.ts";
import { paths } from "./lib/paths.ts";
import { fail } from "./lib/proc.ts";

const args = process.argv.slice(2);
if (args.length === 0) fail("usage: bun run ctl -- <tepatictl args>, e.g. `pact show <id>`");

const project = process.env.TEPATI_CTL_PROJECT;
let cmd: string[];
let cwd: string | undefined;
let env: Record<string, string | undefined> = process.env;
if (project) {
  const opts = parseUpArgs(["--env", project]);
  if ("error" in opts) fail(`TEPATI_CTL_PROJECT: ${opts.error}`);
  env = { ...process.env, TEPATI_ENV: opts.env };
  // Every profile the api's dependencies live in, or compose refuses the service.
  cmd = [...composeArgs(opts.env, paths.root), "--profile", "infra", "--profile", "app", "exec", "-T", "api", "tepatictl", ...args];
} else {
  cmd = ["go", "run", "./cmd/tepatictl", ...args];
  cwd = join(paths.root, "apps", "server");
  env = { ...process.env, ...Object.fromEntries(await readEnvFile(paths.rootEnv)) };
}
const proc = Bun.spawn(cmd, { cwd, stdio: ["inherit", "inherit", "inherit"], env });
process.exit(await proc.exited);
