import { expect, test } from "bun:test";
import { buildCommands, parseBuildArgs } from "./images.ts";

test("--env is required and must be a known environment", () => {
  expect(parseBuildArgs([])).toEqual({ error: "--env is required (staging or production)" });
  expect(parseBuildArgs(["--env", "prod"])).toEqual({ error: 'unknown environment "prod" (use staging or production)' });
  expect(parseBuildArgs(["--env"])).toEqual({ error: "--env is required (staging or production)" });
});

test("both images are built by default, and --only narrows it", () => {
  expect(parseBuildArgs(["--env", "staging"])).toEqual({ env: "staging", only: ["server", "web"] });
  expect(parseBuildArgs(["--env", "production", "--only", "web"])).toEqual({ env: "production", only: ["web"] });
  expect(parseBuildArgs(["--env=staging", "--only=server"])).toEqual({ env: "staging", only: ["server"] });
  expect(parseBuildArgs(["--env", "staging", "--only", "db"])).toEqual({ error: 'unknown image "db" (use server or web)' });
});

test("an unknown flag is an error, so a typo does not build the wrong thing", () => {
  expect(parseBuildArgs(["--env", "staging", "--tag", "x"])).toEqual({ error: 'unknown argument "--tag"' });
});

test("each image is tagged with the environment and the version, built from the repo root", () => {
  const cmds = buildCommands({ env: "staging", only: ["server", "web"] }, "v0.1-3-gabc1234", "/repo");
  expect(cmds).toHaveLength(2);
  expect(cmds[0]).toEqual([
    "docker", "build",
    "-f", "/repo/deploy/docker/server.Dockerfile",
    "-t", "tepati-server:staging",
    "-t", "tepati-server:v0.1-3-gabc1234",
    "--build-arg", "VERSION=v0.1-3-gabc1234",
    "/repo",
  ]);
  expect(cmds[1]?.slice(0, 4)).toEqual(["docker", "build", "-f", "/repo/deploy/docker/web.Dockerfile"]);
  expect(cmds[1]).toContain("tepati-web:staging");
});

test("only the chosen image is built", () => {
  const cmds = buildCommands({ env: "production", only: ["web"] }, "dev", "/repo");
  expect(cmds).toHaveLength(1);
  expect(cmds[0]).toContain("tepati-web:production");
});
