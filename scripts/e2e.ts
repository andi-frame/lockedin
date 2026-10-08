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
    // https://localhost is a deploy:up stack on this machine, with a certificate from Caddy's own CA.
    return (await fetch(url, { signal: AbortSignal.timeout(3000), tls: { rejectUnauthorized: !url.startsWith("https://localhost") } })).ok;
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
// the real service rules (tepatictl seed --scenario today), with new users on every run. One doer
// per spec and per Playwright project, because a spec changes the data it works on (a rest day, a
// submitted proof) and the next one must find the day as the seed left it.
async function seed(scenario: "today" | "passbook" | "review" | "settlement", project: string): Promise<{ email: string; password: string; backer: string }> {
  // The passbook scenario walks a fake clock over past days of an active pact, and the running
  // worker sweeps on the real clock every minute, so once in a while it settles a day the seed was
  // about to act on and the seed stops with "not possible in the current state". Every attempt
  // makes new users, so trying again is safe.
  let last = "";
  for (let attempt = 1; attempt <= 3; attempt++) {
    const seeded = await capture(["bun", join(import.meta.dir, "ctl.ts"), "seed", "--scenario", scenario]);
    const doer = parseSeedLogin(seeded.stdout, "doer");
    const backer = parseSeedLogin(seeded.stdout, "backer");
    if (seeded.code === 0 && doer && backer) {
      log.ok(`seeded ${doer.email} for ${project}`);
      return { ...doer, backer: backer.email };
    }
    last = seeded.stderr || seeded.stdout;
  }
  return fail(`could not seed the ${scenario} scenario for ${project}:
${last}`);
}
// passbook.spec.ts gets the passbook scenario (24 days of printed lines) and moves its pact's clock
// once, so it too has one pact per project.
// settlement.spec.ts gets two pacts that wait for their payout (settlement scenario).
// review.spec.ts gets a backer with three proofs to review (and the doer, for the dispute).
const [todayDesktop, todayMobile, proofDesktop, proofMobile, bookDesktop, bookMobile, reviewDesktop, reviewMobile, settleDesktop, settleMobile] = [
  await seed("today", "today/desktop"),
  await seed("today", "today/mobile"),
  await seed("today", "proof/desktop"),
  await seed("today", "proof/mobile"),
  await seed("passbook", "passbook/desktop"),
  await seed("passbook", "passbook/mobile"),
  await seed("review", "review/desktop"),
  await seed("review", "review/mobile"),
  await seed("settlement", "settlement/desktop"),
  await seed("settlement", "settlement/mobile"),
];

await run(["bunx", "playwright", "test", ...process.argv.slice(2)], {
  cwd: join(paths.root, "apps", "web"),
  env: {
    E2E_TODAY_EMAIL_DESKTOP: todayDesktop.email,
    E2E_TODAY_EMAIL_MOBILE: todayMobile.email,
    E2E_PROOF_EMAIL_DESKTOP: proofDesktop.email,
    E2E_PROOF_EMAIL_MOBILE: proofMobile.email,
    E2E_BOOK_EMAIL_DESKTOP: bookDesktop.email,
    E2E_BOOK_EMAIL_MOBILE: bookMobile.email,
    E2E_REVIEW_BACKER_DESKTOP: reviewDesktop.backer,
    E2E_REVIEW_DOER_DESKTOP: reviewDesktop.email,
    E2E_REVIEW_BACKER_MOBILE: reviewMobile.backer,
    E2E_REVIEW_DOER_MOBILE: reviewMobile.email,
    E2E_SETTLE_BACKER_DESKTOP: settleDesktop.backer,
    E2E_SETTLE_DOER_DESKTOP: settleDesktop.email,
    E2E_SETTLE_BACKER_MOBILE: settleMobile.backer,
    E2E_SETTLE_DOER_MOBILE: settleMobile.email,
    E2E_TODAY_PASSWORD: todayDesktop.password,
  },
});
