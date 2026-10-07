// `bun run ctl -- <tepatictl args>` and `bun run db:seed [-- overdue|invite]`:
// the admin CLI (apps/server/cmd/tepatictl) with .env loaded.
//   bun run ctl -- pact show <id>
//   bun run db:seed                      overdue scenario: an active pact the worker should settle
//   bun run db:seed -- invite            a proposed pact with an open invite link
import { join } from "node:path";
import { readEnvFile } from "./lib/env.ts";
import { paths } from "./lib/paths.ts";
import { fail } from "./lib/proc.ts";

const args = process.argv.slice(2);
if (args.length === 0) fail("usage: bun run ctl -- <tepatictl args>, e.g. `pact show <id>`");

const env = Object.fromEntries(await readEnvFile(paths.rootEnv));
const proc = Bun.spawn(["go", "run", "./cmd/tepatictl", ...args], {
  cwd: join(paths.root, "apps", "server"),
  stdio: ["inherit", "inherit", "inherit"],
  env: { ...process.env, ...env },
});
process.exit(await proc.exited);
