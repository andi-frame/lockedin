import { expect, test } from "bun:test";
import { airCommand } from "./air.ts";

test("airCommand runs air with the given config", () => {
  expect(airCommand("/srv/.air.api.toml", {})).toEqual(["air", "-c", "/srv/.air.api.toml"]);
});

test("AIR_POLL=1 switches air to polling (bind mounts on Windows/macOS have no fs events)", () => {
  const cmd = airCommand("/srv/.air.worker.toml", { AIR_POLL: "1" });
  expect(cmd.slice(0, 3)).toEqual(["air", "-c", "/srv/.air.worker.toml"]);
  expect(cmd).toContain("--build.poll");
  expect(cmd[cmd.indexOf("--build.poll") + 1]).toBe("true");
  expect(cmd[cmd.indexOf("--build.poll_interval") + 1]).toBe("500");
});

test("any other AIR_POLL value leaves polling off", () => {
  expect(airCommand("x.toml", { AIR_POLL: "0" })).toEqual(["air", "-c", "x.toml"]);
  expect(airCommand("x.toml", { AIR_POLL: "" })).toEqual(["air", "-c", "x.toml"]);
});
