// `bun run db:seed [-- overdue|invite|today]`: development data through tepatictl (default: overdue).
import { join } from "node:path";
import { fail } from "./lib/proc.ts";

const scenario = process.argv[2] ?? "overdue";
if (!["overdue", "invite", "today"].includes(scenario)) fail(`unknown scenario "${scenario}" (overdue, invite or today)`);

const proc = Bun.spawn(["bun", join(import.meta.dir, "ctl.ts"), "seed", "--scenario", scenario], {
  stdio: ["inherit", "inherit", "inherit"],
});
process.exit(await proc.exited);
