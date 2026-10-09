import { expect, test } from "bun:test";
import { parseRestoreArgs, restoreCommand } from "./restore.ts";

test("list needs only an environment", () => {
  expect(parseRestoreArgs(["--env", "staging", "list"])).toEqual({ env: "staging", action: "list" });
});

test("check and restore name the backup by its key", () => {
  expect(parseRestoreArgs(["--env", "staging", "check", "daily/2026-10-09.dump"])).toEqual({ env: "staging", action: "check", key: "daily/2026-10-09.dump" });
  expect(parseRestoreArgs(["--env=production", "restore", "weekly/2026-W41.dump", "--live"])).toEqual({ env: "production", action: "restore", key: "weekly/2026-W41.dump", live: true });
});

test("the safety copy a restore leaves behind can be put back with the same command", () => {
  expect(parseRestoreArgs(["--env", "staging", "restore", "pre-restore/20261009T091451Z.dump", "--live"])).toEqual({ env: "staging", action: "restore", key: "pre-restore/20261009T091451Z.dump", live: true });
  expect(parseRestoreArgs(["--env", "staging", "check", "pre-restore/20261009T091451Z.dump"])).toEqual({ env: "staging", action: "check", key: "pre-restore/20261009T091451Z.dump" });
});

test("restoring over the live database must say --live; without it, restore is refused", () => {
  expect(parseRestoreArgs(["--env", "staging", "restore", "daily/2026-10-09.dump"])).toEqual({
    error: "restore replaces the live database: add --live to say you mean it, after stopping the api and the worker (docs/RUNBOOK.md); use `check` to try a backup safely",
  });
});

test("bad input is named", () => {
  expect(parseRestoreArgs([])).toEqual({ error: "--env is required (staging or production)" });
  expect(parseRestoreArgs(["--env", "staging"])).toEqual({ error: "say what to do: list, check <key> or restore <key> --live" });
  expect(parseRestoreArgs(["--env", "staging", "check"])).toEqual({ error: "check needs the backup's key, for example daily/2026-10-09.dump (see `list`)" });
  expect(parseRestoreArgs(["--env", "staging", "check", "../etc/passwd"])).toEqual({ error: 'a backup key looks like daily/YYYY-MM-DD.dump, weekly/YYYY-Www.dump or pre-restore/YYYYMMDDTHHMMSSZ.dump (got "../etc/passwd")' });
  expect(parseRestoreArgs(["--env", "staging", "drop", "x"])).toEqual({ error: 'unknown action "drop" (use list, check or restore)' });
  expect(parseRestoreArgs(["--env", "staging", "list", "--live"])).toEqual({ error: "--live only goes with restore" });
});

test("the restore script runs in the backup image on the deploy's network, and the live flag becomes a confirmation variable", () => {
  const compose = ["docker", "compose", "-p", "tepati-staging"];
  const list = restoreCommand(compose, { env: "staging", action: "list" });
  expect(list.slice(0, 4)).toEqual(compose);
  expect(list).toContain("run");
  expect(list).toContain("--entrypoint");
  expect(list.join(" ")).toContain("restore.sh list");

  const live = restoreCommand(compose, { env: "staging", action: "restore", key: "daily/2026-10-09.dump", live: true });
  expect(live.join(" ")).toContain("-e CONFIRM_RESTORE=tepati");
  expect(live.join(" ")).toContain("restore.sh restore daily/2026-10-09.dump");
  expect(restoreCommand(compose, { env: "staging", action: "check", key: "daily/2026-10-09.dump" }).join(" ")).not.toContain("CONFIRM_RESTORE");
});
