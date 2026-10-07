// `bun run lint`: TypeScript typecheck for scripts, OpenAPI lint, plus gofmt and go vet for the server.
import { existsSync } from "node:fs";
import { join } from "node:path";
import { paths } from "./lib/paths.ts";
import { capture, fail, log, run } from "./lib/proc.ts";

await run(["bunx", "tsc", "-p", join(paths.root, "tsconfig.json"), "--noEmit"]);
log.ok("scripts typecheck");

await run(["bunx", "@redocly/cli", "lint", join(paths.root, "api", "openapi.yaml")]);
log.ok("openapi lint");

const server = join(paths.root, "apps", "server");
if (existsSync(join(server, "go.mod"))) {
  const fmt = await capture(["gofmt", "-l", server]);
  const unformatted = fmt.stdout.trim();
  if (fmt.code !== 0 || unformatted) fail(`gofmt needed:\n${unformatted || fmt.stderr}`);
  log.ok("gofmt");
  const vet = Bun.spawn(["go", "vet", "./..."], { cwd: server, stdio: ["inherit", "inherit", "inherit"] });
  if ((await vet.exited) !== 0) fail("go vet failed");
  log.ok("go vet");
}
