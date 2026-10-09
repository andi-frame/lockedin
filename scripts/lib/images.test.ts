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
    "docker", "build", "--pull",
    "-f", "/repo/deploy/docker/server.Dockerfile",
    "-t", "tepati-server:staging",
    "-t", "tepati-server:v0.1-3-gabc1234",
    "--build-arg", "VERSION=v0.1-3-gabc1234",
    "/repo",
  ]);
  expect(cmds[1]?.slice(0, 5)).toEqual(["docker", "build", "--pull", "-f", "/repo/deploy/docker/web.Dockerfile"]);
  expect(cmds[1]).toContain("tepati-web:staging");
});

test("only the chosen image is built", () => {
  const cmds = buildCommands({ env: "production", only: ["web"] }, "dev", "/repo");
  expect(cmds).toHaveLength(1);
  expect(cmds[0]).toContain("tepati-web:production");
});

// A floating tag such as golang:1.26 is only as new as the copy already on the machine: without
// --pull a build can ship a Go patch release with known standard library vulnerabilities.
test("base images are refreshed on every build", () => {
  for (const cmd of buildCommands({ env: "staging", only: ["server", "web"] }, "dev", "/repo")) expect(cmd).toContain("--pull");
});

test("--pull can be left out, for CI, which has already put fresh base images in place", () => {
  const [withPull] = buildCommands({ env: "staging", only: ["web"] }, "v1", "/repo");
  const [withoutPull] = buildCommands({ env: "staging", only: ["web"] }, "v1", "/repo", { pull: false });
  expect(withPull).toContain("--pull");
  expect(withoutPull).not.toContain("--pull");
  expect(withoutPull).toContain("-f");
});
