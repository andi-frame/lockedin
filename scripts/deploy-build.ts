// `bun run deploy:build -- --env staging [--only server|web]`: builds the production images
// (deploy/docker/*.Dockerfile) and tags them tepati-<image>:<env> and tepati-<image>:<version>.
import { dockerAvailable } from "./lib/compose.ts";
import { buildCommands, parseBuildArgs } from "./lib/images.ts";
import { capture, fail, log, run } from "./lib/proc.ts";
import { paths } from "./lib/paths.ts";

const opts = parseBuildArgs(process.argv.slice(2));
if ("error" in opts) fail(opts.error);
if (!(await dockerAvailable())) fail("Docker is not running. Start Docker Desktop first.");

// `git describe` is the version the binary reports (`tepatictl version`); outside a checkout it stays "dev".
const described = await capture(["git", "describe", "--always", "--dirty"], { cwd: paths.root });
const version = described.code === 0 ? described.stdout.trim() : "dev";

for (const cmd of buildCommands(opts, version, paths.root, { pull: process.env.TEPATI_BUILD_PULL !== "0" })) {
  log.step(`building ${cmd[cmd.indexOf("-t") + 1]} (version ${version})`);
  await run(cmd, { env: { DOCKER_BUILDKIT: "1" } });
}
log.ok("built; `docker image ls | grep tepati` lists them");
