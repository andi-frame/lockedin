// Pre-flight checks for `dev:native`: report every missing dependency at once.
import { connect } from "node:net";

export interface CheckResult {
  name: string;
  ok: boolean;
  detail: string;
}

function hostPort(url: string, fallbackPort: number): { host: string; port: number } {
  const u = new URL(url);
  return { host: u.hostname || "localhost", port: Number(u.port || fallbackPort) };
}

/** Opens a TCP connection, optionally sends a line, and resolves with the first reply chunk. */
function tcpProbe(host: string, port: number, send?: string, timeoutMs = 1500): Promise<string> {
  return new Promise((resolve, reject) => {
    const socket = connect({ host, port });
    const timer = setTimeout(() => {
      socket.destroy();
      reject(new Error("timeout"));
    }, timeoutMs);
    socket.once("connect", () => {
      if (!send) {
        clearTimeout(timer);
        socket.end();
        resolve("");
      } else socket.write(send);
    });
    socket.once("data", (d) => {
      clearTimeout(timer);
      socket.end();
      resolve(d.toString());
    });
    socket.once("error", (e) => {
      clearTimeout(timer);
      reject(e);
    });
  });
}

export async function checkPostgres(databaseUrl: string): Promise<CheckResult> {
  const { host, port } = hostPort(databaseUrl, 5432);
  try {
    await tcpProbe(host, port);
    return { name: "postgres", ok: true, detail: `${host}:${port} reachable` };
  } catch (e) {
    return { name: "postgres", ok: false, detail: `${host}:${port} unreachable (${(e as Error).message}). Install PostgreSQL 18 and point DATABASE_URL at it.` };
  }
}

export async function checkRedis(redisUrl: string): Promise<CheckResult> {
  const { host, port } = hostPort(redisUrl, 6379);
  try {
    const reply = await tcpProbe(host, port, "PING\r\n");
    if (reply.startsWith("+PONG")) return { name: "redis", ok: true, detail: `${host}:${port} PONG` };
    return { name: "redis", ok: false, detail: `${host}:${port} answered ${JSON.stringify(reply.slice(0, 40))}` };
  } catch (e) {
    return { name: "redis", ok: false, detail: `${host}:${port} unreachable (${(e as Error).message}). On Windows use Memurai or Redis in WSL.` };
  }
}

export async function checkBinary(name: string, cmd: string[], hint: string): Promise<CheckResult> {
  try {
    const proc = Bun.spawn(cmd, { stdout: "pipe", stderr: "pipe" });
    const [text, code] = await Promise.all([new Response(proc.stdout).text(), proc.exited]);
    if (code === 0) return { name, ok: true, detail: text.split(/\r?\n/)[0]?.slice(0, 60) ?? "ok" };
    return { name, ok: false, detail: `${cmd[0]} exited ${code}. ${hint}` };
  } catch {
    return { name, ok: false, detail: `${cmd[0]} not found on PATH. ${hint}` };
  }
}
