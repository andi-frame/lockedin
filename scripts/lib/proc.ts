// Small process helpers for the task scripts.
export interface RunResult {
  code: number;
  stdout: string;
  stderr: string;
}

/** Runs a command, captures output, never throws on a non-zero exit. */
export async function capture(cmd: string[], opts: { env?: Record<string, string | undefined>; cwd?: string } = {}): Promise<RunResult> {
  const proc = Bun.spawn(cmd, { stdout: "pipe", stderr: "pipe", env: { ...process.env, ...opts.env } });
  const [stdout, stderr, code] = await Promise.all([
    new Response(proc.stdout).text(),
    new Response(proc.stderr).text(),
    proc.exited,
  ]);
  return { code, stdout, stderr };
}

/** Runs a command with inherited stdio; exits the script if it fails. */
export async function run(cmd: string[], opts: { env?: Record<string, string | undefined>; cwd?: string } = {}): Promise<void> {
  const proc = Bun.spawn(cmd, { cwd: opts.cwd, stdio: ["inherit", "inherit", "inherit"], env: { ...process.env, ...opts.env } });
  const code = await proc.exited;
  if (code !== 0) fail(`command failed (exit ${code}): ${cmd.join(" ")}`);
}

export function fail(message: string): never {
  console.error(`\x1b[31m✖ ${message}\x1b[0m`);
  process.exit(1);
}

export const log = {
  step: (m: string) => console.log(`\x1b[36m›\x1b[0m ${m}`),
  ok: (m: string) => console.log(`\x1b[32m✓\x1b[0m ${m}`),
  skip: (m: string) => console.log(`\x1b[90m= ${m}\x1b[0m`),
  bad: (m: string) => console.log(`\x1b[31m✖\x1b[0m ${m}`),
};
