// `bun run deploy:up -- --env staging|production [--backup|--no-backup]`: brings an environment up
// from the built images (`bun run deploy:build`). The order matters (docs/RUNNING.md §6): infra
// first, then Garage's layout, key and buckets (the API is not ready without its buckets), then
// migrate, api, worker, web and Caddy, then the CORS rule from inside the network.
import { garageInit } from "./garage-init.ts";
import { composeArgs, envFilePath, parseUpArgs, problemsInEnv, profilesFor, projectName } from "./lib/deploy.ts";
import { dockerAvailable } from "./lib/compose.ts";
import { readEnvFile } from "./lib/env.ts";
import { capture, fail, log, run } from "./lib/proc.ts";
import { paths } from "./lib/paths.ts";

const opts = parseUpArgs(process.argv.slice(2));
if ("error" in opts) fail(opts.error);
const { env, backup } = opts;
if (!(await dockerAvailable())) fail("Docker is not running.");

const envFile = envFilePath(env, paths.root);
if (!(await Bun.file(envFile).exists())) {
  fail(`missing ${envFile}\ncopy deploy/env/.env.${env}.example to it and replace every CHANGE_ME (secrets: bun scripts/setup.ts --print-secret hex32)`);
}
const values = await readEnvFile(envFile);
const problems = problemsInEnv(env, values);
if (problems.length) fail(`${envFile} is not ready:\n  - ${problems.join("\n  - ")}`);

for (const image of ["server", "web"]) {
  const found = await capture(["docker", "image", "inspect", `tepati-${image}:${env}`]);
  if (found.code !== 0) fail(`image tepati-${image}:${env} not found; run \`bun run deploy:build -- --env ${env}\` first`);
}

// The compose file reads this to pick the env file and the image tag.
process.env.TEPATI_ENV = env;
const compose = composeArgs(env, paths.root);
const profiles = profilesFor(values, { backup }).flatMap((p) => ["--profile", p]);
const all = [...compose, ...profiles];

log.step(`${projectName(env)}: infra`);
await run([...compose, "--profile", "infra", ...(profiles.includes("mail-sandbox") ? ["--profile", "mail-sandbox"] : []), "up", "-d", "--wait"]);

log.step("garage: layout, key and buckets");
await garageInit({
  compose: async () => all,
  envFile,
  persistTo: [envFile],
  hostCors: false,
  extraBuckets: backup ? [values.get("S3_BUCKET_BACKUPS") || "tepati-backups"] : [],
});

log.step("app: migrate, api, worker, web, caddy");
await run([...all, "up", "-d", "--wait"]);

log.step("storage: CORS for browser uploads");
await run([...all, "exec", "-T", "api", "tepatictl", "storage-init"]);

log.ok(`${env} is up at ${values.get("APP_BASE_URL")}`);
