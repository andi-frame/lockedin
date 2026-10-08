// Pre-flight checks for `dev:native`: report every missing dependency at once.
import { SQL } from "bun";
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

/** Why a native Postgres is not usable, in a sentence that says what to do. Never prints the password. */
export function postgresFailure(err: unknown, databaseUrl: string): string {
  const e = err as { errno?: string; code?: string; message?: string };
  const u = new URL(databaseUrl);
  const where = `${u.hostname || "localhost"}:${u.port || 5432}`;
  const db = u.pathname.replace(/^\//, "") || "postgres";
  if (e.errno === "28P01" || e.errno === "28000") return `${where} rejected the user or password in DATABASE_URL. Fix them in .env.native.`;
  if (e.errno === "3D000") return `${where} is up but has no database "${db}". Create it: createdb -h ${u.hostname} -p ${u.port || 5432} -U ${decodeURIComponent(u.username) || "postgres"} ${db}`;
  if (e.code === "ERR_POSTGRES_CONNECTION_REFUSED") return `${where} is not accepting connections. Start PostgreSQL 18 (or fix the port in .env.native).`;
  return `${where}: ${e.message ?? String(err)}`;
}

/** Native Postgres: a real login, so a wrong password or a missing database is caught here and not as a 500 later. */
export async function checkPostgres(databaseUrl: string): Promise<CheckResult> {
  const sql = new SQL(databaseUrl, { max: 1, connectionTimeout: 3 });
  try {
    const [row] = await sql`select current_setting('server_version') as v`;
    const version = String(row?.v ?? "?");
    const major = Number.parseInt(version, 10);
    const note = major < 18 ? " (the project is developed on PostgreSQL 18)" : "";
    return { name: "postgres", ok: true, detail: `${version}${note}` };
  } catch (e) {
    return { name: "postgres", ok: false, detail: postgresFailure(e, databaseUrl) };
  } finally {
    await sql.close().catch(() => {});
  }
}

/** The login limiter uses EXPIRE ... NX, which Redis only has from 7.0 (Memurai 4 and Redis 8 do). */
export function redisVersionProblem(info: string): string | null {
  const m = /redis_version:(\d+)\.(\d+)\.(\d+)/.exec(info);
  if (!m) return "could not read the version from INFO. Is this a Redis server?";
  if (Number(m[1]) >= 7) return null;
  return `version ${m[1]}.${m[2]}.${m[3]} is too old: Tepati needs 7.0 or newer (EXPIRE ... NX). Use Memurai or a newer Redis in WSL.`;
}

export async function checkRedis(redisUrl: string): Promise<CheckResult> {
  const { host, port } = hostPort(redisUrl, 6379);
  try {
    const reply = await tcpProbe(host, port, "PING\r\n");
    if (!reply.startsWith("+PONG")) return { name: "redis", ok: false, detail: `${host}:${port} answered ${JSON.stringify(reply.slice(0, 40))}` };
    const info = await tcpProbe(host, port, "INFO server\r\n");
    const problem = redisVersionProblem(info);
    if (problem) return { name: "redis", ok: false, detail: `${host}:${port} ${problem}` };
    return { name: "redis", ok: true, detail: `${host}:${port} ${/redis_version:(\S+)/.exec(info)?.[1] ?? ""}`.trim() };
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
