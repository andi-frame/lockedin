// `bun run setup`: create local env files from the committed templates.
// Idempotent: an existing file is never overwritten (RUNNING.md §2).
import { generateSecret, materialize, parseEnv, type SecretKind } from "./lib/env.ts";
import { paths } from "./lib/paths.ts";

const args = process.argv.slice(2);
const printIdx = args.indexOf("--print-secret");
if (printIdx !== -1) {
  const kind = (args[printIdx + 1] ?? "hex32") as SecretKind;
  console.log(generateSecret(kind));
  process.exit(0);
}

async function ensure(target: string, template: string, fromRoot?: Map<string, string>) {
  const rel = target.slice(paths.root.length + 1).replaceAll("\\", "/");
  if (await Bun.file(target).exists()) {
    console.log(`${rel}: exists, skipped`);
    return;
  }
  const text = materialize(await Bun.file(template).text(), { fromRoot });
  await Bun.write(target, text);
  console.log(`${rel}: created`);
}

await ensure(paths.rootEnv, paths.envTemplate);
// Docker dev shares Postgres credentials and Garage secrets with the root .env, so
// switching between hybrid and docker modes keeps using the same volumes.
const rootEnv = parseEnv(await Bun.file(paths.rootEnv).text());
await ensure(paths.devEnv, paths.devEnvTemplate, rootEnv);

console.log("Staging/production env files are created by hand from deploy/env/.env.{staging,production}.example.");
