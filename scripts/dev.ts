// `bun run dev:{docker,hybrid,native,apps}`: one entry point for every run mode (ADR-0008).
import { existsSync } from "node:fs";
import { join } from "node:path";
import { airCommand } from "./lib/air.ts";
import { checkBinary, checkPostgres, checkRedis } from "./lib/checks.ts";
import { readEnvFile } from "./lib/env.ts";
import { paths } from "./lib/paths.ts";
import { fail, log, run } from "./lib/proc.ts";
import { supervise, type ProcSpec } from "./lib/supervisor.ts";

type Mode = "docker" | "hybrid" | "native" | "apps";
const mode = (process.argv[2] ?? "hybrid") as Mode;
if (!["docker", "hybrid", "native", "apps"].includes(mode)) fail(`unknown mode "${mode}"`);

const web = join(paths.root, "apps", "web");
const server = join(paths.root, "apps", "server");

/** App processes that exist in the repo right now; the rest are reported with their PLAN task. */
function appSpecs(env: Record<string, string>): { specs: ProcSpec[]; missing: string[] } {
  const specs: ProcSpec[] = [];
  const missing: string[] = [];
  if (existsSync(join(web, "package.json"))) {
    specs.push({ name: "web", cwd: web, env, cmd: ["bun", "--bun", "next", "dev", "-p", env.WEB_PORT ?? "3000"] });
  } else missing.push("web (PLAN 5.1)");
  for (const name of ["api", "worker"] as const) {
    const air = join(server, `.air.${name}.toml`);
    if (existsSync(join(server, "go.mod")) && existsSync(air)) {
      specs.push({ name, cwd: server, env, cmd: airCommand(air, env) });
    } else missing.push(`${name} (.air.${name}.toml)`);
  }
  return { specs, missing };
}

async function loadEnv(): Promise<Record<string, string>> {
  if (!existsSync(paths.rootEnv)) fail("missing .env, run `bun run setup` first");
  const env = Object.fromEntries(await readEnvFile(paths.rootEnv));
  // `.env.native` (optional, gitignored) overrides hosts/ports for natively installed services.
  if (mode === "native") Object.assign(env, Object.fromEntries(await readEnvFile(join(paths.root, ".env.native"))));
  return env;
}

async function nativePreflight(env: Record<string, string>) {
  const results = await Promise.all([
    checkPostgres(env.DATABASE_URL ?? "postgres://localhost:5432"),
    checkRedis(env.REDIS_URL ?? "redis://localhost:6379"),
    checkBinary("ffmpeg", [env.FFMPEG_PATH ?? "ffmpeg", "-version"], "Install ffmpeg (Windows: winget install Gyan.FFmpeg)."),
    checkBinary("vips", [env.VIPS_PATH ?? "vips", "--version"], "Install the libvips CLI and put `vips` on PATH."),
  ]);
  for (const r of results) (r.ok ? log.ok : log.bad)(`${r.name}: ${r.detail}`);
  const failed = results.filter((r) => !r.ok);
  if (failed.length) fail(`native mode needs: ${failed.map((r) => r.name).join(", ")} (see docs/RUNNING.md §1)`);
}

async function startApps(env: Record<string, string>) {
  const { specs, missing } = appSpecs(env);
  if (missing.length) log.skip(`apps not yet scaffolded: ${missing.join(", ")}`);
  if (!specs.length) {
    log.ok("nothing to run yet; infra (if any) keeps running in the background");
    return;
  }
  process.exit(await supervise(specs));
}

const env = await loadEnv();
switch (mode) {
  case "hybrid":
    await run(["bun", join(paths.root, "scripts", "infra.ts"), "up"]);
    await startApps(env);
    break;
  case "native":
    env.STORAGE_DRIVER = "fs"; // Garage has no Windows build; ADR-0008
    await nativePreflight(env);
    await startApps(env);
    break;
  case "apps":
    await startApps(env);
    break;
  case "docker": {
    if (!existsSync(paths.composeDev)) {
      log.skip("app containers arrive with deploy/compose.dev.yaml (PLAN 7.2); starting infra only");
      await run(["bun", join(paths.root, "scripts", "infra.ts"), "up"]);
      break;
    }
    if (!existsSync(paths.devEnv)) fail("missing deploy/env/.env.dev, run `bun run setup` first");
    await run(["bun", join(paths.root, "scripts", "infra.ts"), "up"]);
    await run([
      "docker", "compose", "-f", paths.composeBase, "-f", paths.composeDev,
      "--env-file", paths.devEnv, "--profile", "infra", "--profile", "app", "up", "--build",
    ]);
    break;
  }
}
