// `bun run db:{migrate,rollback,status,new}`: goose against DATABASE_URL from .env.
//   bun run db:migrate                 apply all pending migrations
//   bun run db:rollback [-- --all]     undo the last migration (or all of them; dev only)
//   bun run db:status
//   bun run db:new -- <name>           create db/migrations/<timestamp>_<name>.sql
import { join } from "node:path";
import { pickDatabaseUrl, readRootEnv } from "./lib/env.ts";
import { paths } from "./lib/paths.ts";
import { fail, run } from "./lib/proc.ts";

const [action, ...rest] = process.argv.slice(2);
const dir = join(paths.root, "apps", "server", "db", "migrations");
const tools = join(paths.root, "apps", "server", "tools");

// TEPATI_NATIVE=1 aims it at the native database from `.env.native` (dev:native sets it).
const env = await readRootEnv(paths.rootEnv, paths.nativeEnv);
const dsn = pickDatabaseUrl(process.env, env, process.env.TEPATI_NATIVE === "1");
if (!dsn) fail("DATABASE_URL is not set (run `bun run setup`)");

async function goose(...args: string[]) {
  const proc = Bun.spawn(["go", "tool", "goose", "-dir", dir, ...args], {
    cwd: tools,
    stdio: ["inherit", "inherit", "inherit"],
    env: { ...process.env, GOOSE_DRIVER: "postgres", GOOSE_DBSTRING: dsn },
  });
  const code = await proc.exited;
  if (code !== 0) process.exit(code);
}

switch (action) {
  case "migrate":
    await goose("up");
    break;
  case "rollback":
    if (rest.includes("--all")) {
      if (env.get("APP_ENV") === "production") fail("refusing to roll back everything in production");
      await goose("reset");
    } else await goose("down");
    break;
  case "status":
    await goose("status");
    break;
  case "new": {
    const name = rest[0];
    if (!name || !/^[a-z0-9_]+$/.test(name)) fail("usage: bun run db:new -- <snake_case_name>");
    await goose("create", name, "sql");
    break;
  }
  default:
    await run(["bun", "scripts/todo.ts", `db:${action ?? ""}`, "1.2"]);
}
