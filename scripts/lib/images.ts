// What `bun run deploy:build` builds: the two production images, tagged for an environment.
export const ENVIRONMENTS = ["staging", "production"] as const;
export const IMAGES = ["server", "web"] as const;

export type Environment = (typeof ENVIRONMENTS)[number];
export type Image = (typeof IMAGES)[number];
export type BuildOptions = { env: Environment; only: Image[] };

const oneOf = <T extends string>(list: readonly T[], v: string): v is T => (list as readonly string[]).includes(v);

/** Reads `--env <name>` and `--only <image>`; anything else is an error, so a typo never builds the wrong thing. */
export function parseBuildArgs(argv: string[]): BuildOptions | { error: string } {
  let env: string | undefined;
  let only: string | undefined;
  for (let i = 0; i < argv.length; i++) {
    const [flag = "", inline] = (argv[i] ?? "").split(/=(.*)/s, 2);
    if (flag !== "--env" && flag !== "--only") return { error: `unknown argument "${flag}"` };
    const value = inline ?? (argv[i + 1]?.startsWith("--") ? undefined : argv[++i]);
    if (flag === "--env") env = value;
    else only = value;
  }
  if (!env) return { error: "--env is required (staging or production)" };
  if (!oneOf(ENVIRONMENTS, env)) return { error: `unknown environment "${env}" (use staging or production)` };
  if (only === undefined) return { env, only: [...IMAGES] };
  if (!oneOf(IMAGES, only)) return { error: `unknown image "${only}" (use server or web)` };
  return { env, only: [only] };
}

/**
 * One `docker build` per image, from the repo root (both Dockerfiles copy from `apps/` and the
 * lockfile). `--pull` refreshes the base images: a floating tag is only as new as the local copy,
 * and an old Go patch release carries known standard library vulnerabilities. Forward slashes on
 * purpose: Docker accepts them on Windows too.
 */
export function buildCommands(opts: BuildOptions, version: string, root: string): string[][] {
  return opts.only.map((image) => [
    "docker", "build", "--pull",
    "-f", `${root}/deploy/docker/${image}.Dockerfile`,
    "-t", `tepati-${image}:${opts.env}`,
    "-t", `tepati-${image}:${version}`,
    "--build-arg", `VERSION=${version}`,
    root,
  ]);
}
