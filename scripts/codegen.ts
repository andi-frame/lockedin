// `bun run codegen`: regenerate code from contracts.
//   sqlc:  apps/server/db/{migrations,queries} → apps/server/internal/store
//   (OpenAPI → Go + TS joins in PLAN 2.1)
//   --check  fail if regenerating changes any tracked file (CI freshness check)
import { join } from "node:path";
import { paths } from "./lib/paths.ts";
import { capture, fail, log } from "./lib/proc.ts";

const check = process.argv.includes("--check");
const server = join(paths.root, "apps", "server");
const tools = join(server, "tools");

async function tool(name: string, ...args: string[]) {
  const proc = Bun.spawn(["go", "tool", name, ...args], { cwd: tools, stdio: ["inherit", "inherit", "inherit"] });
  if ((await proc.exited) !== 0) fail(`${name} failed`);
}

await tool("sqlc", "generate", "-f", join(server, "sqlc.yaml"));
log.ok("sqlc");

if (check) {
  // Compare the regenerated files with what is staged/committed, and catch new untracked files.
  const generated = ["apps/server/internal/store"];
  const changed = await capture(["git", "-C", paths.root, "diff", "--name-only", "--", ...generated]);
  const untracked = await capture(["git", "-C", paths.root, "ls-files", "--others", "--exclude-standard", "--", ...generated]);
  const stale = (changed.stdout + untracked.stdout).trim();
  if (stale) fail(`generated code is stale; run \`bun run codegen\` and commit:\n${stale}`);
  log.ok("generated code is fresh");
}
