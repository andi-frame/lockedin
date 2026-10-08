// `bun run deploy:env-local`: writes deploy/env/.env.staging for a deploy on this machine or a CI
// runner (https://localhost, Mailpit, fresh secrets). It refuses to overwrite a file that exists.
import { renderLocalStagingEnv } from "./lib/localenv.ts";
import { generateSecret } from "./lib/env.ts";
import { envFilePath } from "./lib/deploy.ts";
import { fail, log } from "./lib/proc.ts";
import { paths } from "./lib/paths.ts";

const target = envFilePath("staging", paths.root);
if (await Bun.file(target).exists()) fail(`${target} already exists; delete it first if you want a new one`);
const template = await Bun.file(`${paths.envDir}/.env.staging.example`).text();
await Bun.write(target, renderLocalStagingEnv(template, generateSecret));
log.ok(`wrote ${target} (https://localhost, Mailpit); next: bun run deploy:build -- --env staging, then deploy:up`);
