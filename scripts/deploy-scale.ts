// `bun run deploy:scale -- --env staging api=3 web=2 worker=2`: changes how many copies of the
// stateless services run. Nothing else is touched or recreated; Caddy finds new api and web
// containers through DNS (deploy/caddy/Caddyfile).
import { composeArgs, envFilePath, parseScaleArgs, scaleCommand } from "./lib/deploy.ts";
import { dockerAvailable } from "./lib/compose.ts";
import { fail, log, run } from "./lib/proc.ts";
import { paths } from "./lib/paths.ts";

const opts = parseScaleArgs(process.argv.slice(2));
if ("error" in opts) fail(opts.error);
if (!(await dockerAvailable())) fail("Docker is not running.");
if (!(await Bun.file(envFilePath(opts.env, paths.root)).exists())) fail(`missing ${envFilePath(opts.env, paths.root)}`);

process.env.TEPATI_ENV = opts.env;
const compose = [...composeArgs(opts.env, paths.root), "--profile", "app"];
log.step(`scaling ${Object.entries(opts.scale).map(([s, n]) => `${s}=${n}`).join(" ")}`);
await run(scaleCommand(compose, opts.scale));
await run([...compose, "ps", "--format", "table {{.Service}}\t{{.Status}}"]);
