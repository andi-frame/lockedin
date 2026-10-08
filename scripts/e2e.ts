// `bun run test:e2e`: runs the Playwright specs in apps/web against a stack that is already up
// (`bun run dev:hybrid`). Extra arguments go to Playwright, e.g. `bun run test:e2e -g login`.
import { join } from "node:path";
import { paths } from "./lib/paths.ts";
import { capture, fail, log, run } from "./lib/proc.ts";
import { parseSeedLogin } from "./lib/seed-output.ts";

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

// today.spec.ts signs in as a doer who has every check-in state at once. The states come from
// the real service rules (tepatictl seed --scenario today), new users on every run.
const seeded = await capture(["bun", join(import.meta.dir, "ctl.ts"), "seed", "--scenario", "today"]);
const doer = parseSeedLogin(seeded.stdout, "doer");
if (seeded.code !== 0 || !doer) fail(`could not seed the today scenario:
${seeded.stderr || seeded.stdout}`);
log.ok(`seeded ${doer.email}`);

await run(["bunx", "playwright", "test", ...process.argv.slice(2)], {
  cwd: join(paths.root, "apps", "web"),
  env: { E2E_TODAY_EMAIL: doer.email, E2E_TODAY_PASSWORD: doer.password },
});
