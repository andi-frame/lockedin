// Runs several long-lived processes with prefixed, coloured output.
// If any child exits on its own, the rest are stopped and the supervisor returns
// that child's exit code (non-zero if it crashed), so failures never hide.

export interface ProcSpec {
  name: string;
  cmd: string[];
  cwd?: string;
  env?: Record<string, string | undefined>;
}

const COLORS = [36, 35, 33, 32, 34, 31];

async function pipeLines(stream: ReadableStream<Uint8Array>, prefix: string, out: (line: string) => void) {
  const decoder = new TextDecoder();
  let buf = "";
  for await (const chunk of stream) {
    buf += decoder.decode(chunk, { stream: true });
    let nl: number;
    while ((nl = buf.indexOf("\n")) !== -1) {
      out(`${prefix} ${buf.slice(0, nl).replace(/\r$/, "")}`);
      buf = buf.slice(nl + 1);
    }
  }
  if (buf) out(`${prefix} ${buf}`);
}

export interface SuperviseOptions {
  /** Line sink, replaceable in tests. */
  out?: (line: string) => void;
  /** Resolves when the user asks to stop (Ctrl+C); defaults to SIGINT/SIGTERM. */
  stopSignal?: Promise<void>;
}

export async function supervise(specs: ProcSpec[], opts: SuperviseOptions = {}): Promise<number> {
  const out = opts.out ?? ((l: string) => console.log(l));
  const width = Math.max(...specs.map((s) => s.name.length));
  const children = specs.map((spec, i) => {
    const prefix = `\x1b[${COLORS[i % COLORS.length]}m${spec.name.padEnd(width)} │\x1b[0m`;
    const child = Bun.spawn(spec.cmd, {
      cwd: spec.cwd,
      env: { ...process.env, FORCE_COLOR: "1", ...spec.env },
      stdout: "pipe",
      stderr: "pipe",
      stdin: "ignore",
    });
    const piped = Promise.all([pipeLines(child.stdout, prefix, out), pipeLines(child.stderr, prefix, out)]);
    return { spec, child, piped };
  });

  let stopping = false;
  const stopAll = () => {
    stopping = true;
    for (const c of children) if (c.child.exitCode === null) c.child.kill();
  };

  const stop =
    opts.stopSignal ??
    new Promise<void>((resolve) => {
      process.once("SIGINT", () => resolve());
      process.once("SIGTERM", () => resolve());
    });

  const firstExit = Promise.race(
    children.map(async (c) => ({ c, code: await c.child.exited })),
  );
  const winner = await Promise.race([firstExit, stop.then(() => null)]);

  if (winner === null) {
    out("\x1b[90mstopping…\x1b[0m");
    stopAll();
    await Promise.all(children.map((c) => c.child.exited));
    await Promise.all(children.map((c) => c.piped));
    return 0;
  }

  if (!stopping) {
    const verdict = winner.code === 0 ? "exited" : `crashed (exit ${winner.code})`;
    out(`\x1b[31m${winner.c.spec.name} ${verdict}; stopping the others\x1b[0m`);
  }
  stopAll();
  await Promise.all(children.map((c) => c.child.exited));
  await Promise.all(children.map((c) => c.piped));
  return winner.code === 0 ? 1 : winner.code; // a dev server exiting by itself is still a failure
}
