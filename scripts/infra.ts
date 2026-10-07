// `bun run infra:{up,down,reset,status}`: Postgres, Redis, Garage, Mailpit in Docker.
import { composeBase, dockerAvailable } from "./lib/compose.ts";
import { fail, log, run } from "./lib/proc.ts";
import { garageInit } from "./garage-init.ts";

const [action = "up", ...rest] = process.argv.slice(2);

if (!(await dockerAvailable())) fail("Docker is not running. Start Docker Desktop (or use `bun run dev:native`).");
const compose = [...(await composeBase()), "--profile", "infra"];

switch (action) {
  case "up":
    log.step("starting infra (postgres, redis, garage, mailpit)");
    await run([...compose, "up", "-d", "--wait"]);
    await garageInit();
    log.ok("infra ready");
    break;
  case "down":
    await run([...compose, "down"]);
    log.ok("infra stopped (volumes kept)");
    break;
  case "reset": {
    if (!rest.includes("--yes")) {
      const answer = prompt("This deletes the Postgres, Redis and Garage volumes. Type 'reset' to continue:");
      if (answer !== "reset") fail("aborted");
    }
    await run([...compose, "down", "--volumes"]);
    log.ok("infra stopped and volumes deleted (S3 keys in .env stay valid: garage:init re-imports them)");
    break;
  }
  case "status":
    await run([...compose, "ps"]);
    break;
  default:
    fail(`unknown action "${action}" (use up | down | reset | status)`);
}
