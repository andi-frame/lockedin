// Planning for `bun run restore`: which backup to look at or restore, and the command that runs
// deploy/backup/restore.sh in the backup image on the deploy's compose network.
import { ENVIRONMENTS, type Environment } from "./images.ts";

export type RestoreAction = "list" | "check" | "restore";
export type RestoreOptions = { env: Environment; action: RestoreAction; key?: string; live?: boolean };

const KEY = /^(daily\/\d{4}-\d{2}-\d{2}|weekly\/\d{4}-W\d{2}|pre-restore\/\d{8}T\d{6}Z)\.dump$/;

export function parseRestoreArgs(argv: string[]): RestoreOptions | { error: string } {
  let env: string | undefined;
  let live = false;
  const rest: string[] = [];
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i] ?? "";
    if (arg === "--live") live = true;
    else if (arg === "--env") env = argv[++i];
    else if (arg.startsWith("--env=")) env = arg.slice(6);
    else if (arg.startsWith("--")) return { error: `unknown argument "${arg}"` };
    else rest.push(arg);
  }
  if (!env) return { error: "--env is required (staging or production)" };
  if (!(ENVIRONMENTS as readonly string[]).includes(env)) return { error: `unknown environment "${env}" (use staging or production)` };
  const [action, key] = rest;
  if (!action) return { error: "say what to do: list, check <key> or restore <key> --live" };
  if (action !== "list" && action !== "check" && action !== "restore") return { error: `unknown action "${action}" (use list, check or restore)` };
  if (live && action !== "restore") return { error: "--live only goes with restore" };
  if (action === "list") return { env: env as Environment, action };
  if (!key) return { error: `${action} needs the backup's key, for example daily/2026-10-09.dump (see \`list\`)` };
  if (!KEY.test(key)) return { error: `a backup key looks like daily/YYYY-MM-DD.dump, weekly/YYYY-Www.dump or pre-restore/YYYYMMDDTHHMMSSZ.dump (got "${key}")` };
  if (action === "restore" && !live) {
    return { error: "restore replaces the live database: add --live to say you mean it, after stopping the api and the worker (docs/RUNBOOK.md); use `check` to try a backup safely" };
  }
  return action === "restore" ? { env: env as Environment, action, key, live: true } : { env: env as Environment, action, key };
}

/** `compose` already carries the project, files, env file and profiles. */
export function restoreCommand(compose: string[], opts: RestoreOptions): string[] {
  return [
    ...compose, "run", "--rm", "-T",
    ...(opts.live ? ["-e", "CONFIRM_RESTORE=tepati"] : []),
    "--entrypoint", "sh", "backup", "/usr/local/bin/restore.sh", opts.action, ...(opts.key ? [opts.key] : []),
  ];
}
