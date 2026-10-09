// `bun run restore -- --env staging list | check <key> | restore <key> --live`: the backups in the
// Garage bucket. `check` is the restore drill (a scratch database, nothing live is touched);
// `restore ... --live` replaces the live database (docs/RUNBOOK.md).
import { composeArgs, envFilePath, profilesFor } from "./lib/deploy.ts";
import { dockerAvailable } from "./lib/compose.ts";
import { readEnvFile } from "./lib/env.ts";
import { parseRestoreArgs, restoreCommand } from "./lib/restore.ts";
import { fail, run } from "./lib/proc.ts";
import { paths } from "./lib/paths.ts";

const opts = parseRestoreArgs(process.argv.slice(2));
if ("error" in opts) fail(opts.error);
if (!(await dockerAvailable())) fail("Docker is not running.");
if (!(await Bun.file(envFilePath(opts.env, paths.root)).exists())) fail(`missing ${envFilePath(opts.env, paths.root)}`);

process.env.TEPATI_ENV = opts.env; // the compose file reads it; passed to the child explicitly below, Bun.spawn does not see later changes
const values = await readEnvFile(envFilePath(opts.env, paths.root));
// The backup service lives in the `backup` profile and depends on infra: compose needs every profile.
const profiles = [...profilesFor(values, { backup: true })].flatMap((p) => ["--profile", p]);
const compose = [...composeArgs(opts.env, paths.root), ...profiles];
// The image carries restore.sh, so make sure it is the current one before using it.
await run([...compose, "build", "--quiet", "backup"]);
const proc = Bun.spawn(restoreCommand(compose, opts), { stdio: ["inherit", "inherit", "inherit"], env: { ...process.env } });
process.exit(await proc.exited);
