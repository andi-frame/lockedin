import { paths } from "./paths.ts";
import { fail } from "./proc.ts";

/** `docker compose` with the base file and the root .env used for interpolation. */
export async function composeBase(): Promise<string[]> {
  if (!(await Bun.file(paths.rootEnv).exists())) fail("missing .env, run `bun run setup` first");
  return ["docker", "compose", "-f", paths.composeBase, "--env-file", paths.rootEnv];
}

export async function dockerAvailable(): Promise<boolean> {
  const proc = Bun.spawn(["docker", "info", "--format", "{{.ServerVersion}}"], { stdout: "ignore", stderr: "ignore" });
  return (await proc.exited) === 0;
}
