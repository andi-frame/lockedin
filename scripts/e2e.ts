// `bun run test:e2e`: runs the Playwright specs in apps/web against a stack that is already up
// (`bun run dev:hybrid`). Extra arguments go to Playwright, e.g. `bun run test:e2e -g login`.
import { join } from "node:path";
import { paths } from "./lib/paths.ts";
import { fail, log, run } from "./lib/proc.ts";

const web = process.env.E2E_BASE_URL ?? "http://localhost:3000";
const api = process.env.E2E_API_URL ?? "http://localhost:8080";

async function reachable(url: string): Promise<boolean> {
  try {
    return (await fetch(url, { signal: AbortSignal.timeout(3000) })).ok;
  } catch {
    return false;
  }
}

const [webUp, apiUp] = await Promise.all([reachable(`${web}/login`), reachable(`${api}/readyz`)]);
if (!webUp || !apiUp) {
  fail(
    `the stack is not up (web ${webUp ? "ok" : `unreachable at ${web}`}, api ${apiUp ? "ok" : `unreachable at ${api}`}). ` +
      "Start it with `bun run dev:hybrid` and run this again.",
  );
}
log.ok("web and api are up");
log.skip("the API allows 10 auth requests a minute per IP; wait a minute between full runs");

await run(["bunx", "playwright", "test", ...process.argv.slice(2)], { cwd: join(paths.root, "apps", "web") });
