// Minimal dotenv handling shared by the task scripts. It is our own parser so that
// templates keep their comments and layout when copied, and so every mode
// (docker, hybrid, native) reads env files the same way.
import { randomBytes } from "node:crypto";

export type EnvMap = Map<string, string>;

const LINE = /^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/;

/** Strips quotes, or an inline ` # comment` from an unquoted value. */
function cleanValue(raw: string): string {
  const v = raw.trim();
  const quote = v[0];
  if ((quote === '"' || quote === "'") && v.lastIndexOf(quote) > 0) {
    return v.slice(1, v.lastIndexOf(quote));
  }
  if (v.startsWith("#")) return ""; // `KEY=   # comment` means an empty value
  const hash = v.search(/\s#/);
  return (hash === -1 ? v : v.slice(0, hash)).trim();
}

export function parseEnv(text: string): EnvMap {
  const out: EnvMap = new Map();
  for (const line of text.split(/\r?\n/)) {
    if (/^\s*#/.test(line)) continue;
    const m = LINE.exec(line);
    if (m) out.set(m[1]!, cleanValue(m[2]!));
  }
  return out;
}

export type SecretKind = "hex16" | "hex32" | "base64_32";

export function generateSecret(kind: SecretKind): string {
  switch (kind) {
    case "hex16":
      return randomBytes(16).toString("hex");
    case "hex32":
      return randomBytes(32).toString("hex");
    case "base64_32":
      return randomBytes(32).toString("base64url");
  }
}

/** Replaces one key's value in env text, preserving its inline comment. Appends the key if absent. */
export function setEnvValue(text: string, key: string, value: string): string {
  const lines = text.split(/\r?\n/);
  let found = false;
  const next = lines.map((line) => {
    const m = LINE.exec(line);
    if (!m || m[1] !== key || /^\s*#/.test(line)) return line;
    found = true;
    const comment = /(?:^|\s)(#.*)$/.exec(m[2]!.startsWith('"') ? "" : m[2]!);
    const pad = comment ? " ".repeat(Math.max(1, 46 - key.length - value.length - 1)) : "";
    return `${key}=${value}${comment ? pad + comment[1] : ""}`;
  });
  if (!found) next.splice(next.at(-1) === "" ? next.length - 1 : next.length, 0, `${key}=${value}`);
  return next.join("\n");
}

export interface MaterializeOptions {
  /** Values for `__from_root__` placeholders, looked up by key. */
  fromRoot?: EnvMap;
}

/**
 * Turns a template into a concrete env file:
 * `__generate:<kind>__` becomes a fresh secret, `__from_root__` copies the same key
 * from the root env, and `${VAR}` references are expanded against the file's own
 * final values so that every consumer (Go, Bun, Compose) reads literal values.
 */
export function materialize(template: string, opts: MaterializeOptions = {}): string {
  let text = template;
  const keys = [...parseEnv(template).entries()];
  for (const [key, value] of keys) {
    const gen = /^__generate:(hex16|hex32|base64_32)__$/.exec(value);
    if (gen) text = setEnvValue(text, key, generateSecret(gen[1] as SecretKind));
    else if (value === "__from_root__") {
      const v = opts.fromRoot?.get(key);
      if (v === undefined) throw new Error(`${key} is __from_root__ but the root .env has no ${key}`);
      text = setEnvValue(text, key, v);
    }
  }
  const resolved = parseEnv(text);
  for (const [key, value] of resolved) {
    if (!value.includes("${")) continue;
    const expanded = value.replace(/\$\{([A-Za-z_][A-Za-z0-9_]*)\}/g, (_, ref: string) => {
      const v = resolved.get(ref);
      if (v === undefined) throw new Error(`${key} references undefined \${${ref}}`);
      return v;
    });
    text = setEnvValue(text, key, expanded);
  }
  return text;
}

export async function readEnvFile(path: string): Promise<EnvMap> {
  const f = Bun.file(path);
  return (await f.exists()) ? parseEnv(await f.text()) : new Map();
}

/**
 * Native mode (no Docker): `.env.native` overrides the hosts and ports of `.env`, and files go to the
 * fs driver because Garage has no Windows build (ADR-0008). Returns a new map.
 */
export function layerNativeEnv(base: EnvMap, native: EnvMap): EnvMap {
  const out: EnvMap = new Map(base);
  for (const [k, v] of native) out.set(k, v);
  out.set("STORAGE_DRIVER", "fs");
  return out;
}

/**
 * The root `.env` as the scripts should see it: with `TEPATI_NATIVE=1` it is layered with
 * `.env.native`, so `db:migrate`, `ctl` and the e2e seeds talk to the native database and not to the
 * Docker one.
 */
export async function readRootEnv(rootEnv: string, nativeEnv: string, native = process.env.TEPATI_NATIVE === "1"): Promise<EnvMap> {
  const base = await readEnvFile(rootEnv);
  return native ? layerNativeEnv(base, await readEnvFile(nativeEnv)) : base;
}

/** Which DATABASE_URL a script uses. In native mode the layered files win: Bun puts the Docker URL from `.env` into process.env. */
export function pickDatabaseUrl(processEnv: Record<string, string | undefined>, files: EnvMap, native: boolean): string | undefined {
  const fromFiles = files.get("DATABASE_URL");
  return native ? fromFiles || undefined : processEnv.DATABASE_URL || fromFiles || undefined;
}
