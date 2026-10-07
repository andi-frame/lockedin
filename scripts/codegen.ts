// `bun run codegen`: regenerate code from contracts.
//   sqlc:     apps/server/db/{migrations,queries} → apps/server/internal/store
//   openapi:  api/openapi.yaml → Go strict server (internal/http/api) and TS types (apps/web/src/lib/api/schema.d.ts)
//   --check   fail if regenerating changes any tracked file (CI freshness check)
import { mkdirSync } from "node:fs";
import { join } from "node:path";
import { paths } from "./lib/paths.ts";
import { capture, fail, log } from "./lib/proc.ts";

const check = process.argv.includes("--check");
const server = join(paths.root, "apps", "server");
const tools = join(server, "tools");
const spec = join(paths.root, "api", "openapi.yaml");
const goApi = join(server, "internal", "http", "api", "api.gen.go");
const tsSchema = join(paths.root, "apps", "web", "src", "lib", "api", "schema.d.ts");

async function spawn(cmd: string[], cwd: string, label: string) {
  const proc = Bun.spawn(cmd, { cwd, stdio: ["inherit", "inherit", "inherit"] });
  if ((await proc.exited) !== 0) fail(`${label} failed`);
}

const tool = (name: string, ...args: string[]) => spawn(["go", "tool", name, ...args], tools, name);

await tool("sqlc", "generate", "-f", join(server, "sqlc.yaml"));
log.ok("sqlc");

mkdirSync(join(server, "internal", "http", "api"), { recursive: true });
await tool("oapi-codegen", "-config", join(server, "oapi-codegen.yaml"), "-o", goApi, spec);
log.ok("oapi-codegen (Go strict server)");

mkdirSync(join(paths.root, "apps", "web", "src", "lib", "api"), { recursive: true });
await spawn(["bunx", "openapi-typescript", spec, "-o", tsSchema], paths.root, "openapi-typescript");
log.ok("openapi-typescript (web schema.d.ts)");

if (check) {
  // (apps/web/src/lib/api also holds the hand-written client, so only schema.d.ts is checked.)
  // Compare the regenerated files with what is staged/committed, and catch new untracked files.
  const generated = ["apps/server/internal/store", "apps/server/internal/http/api", "apps/web/src/lib/api/schema.d.ts"];
  const changed = await capture(["git", "-C", paths.root, "diff", "--name-only", "--", ...generated]);
  const untracked = await capture(["git", "-C", paths.root, "ls-files", "--others", "--exclude-standard", "--", ...generated]);
  const stale = (changed.stdout + untracked.stdout).trim();
  if (stale) fail(`generated code is stale; run \`bun run codegen\` and commit:\n${stale}`);
  log.ok("generated code is fresh");
}
