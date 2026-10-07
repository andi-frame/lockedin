import { join, resolve } from "node:path";

/** Repo root. TEPATI_ROOT overrides it so tests can run scripts against a temp directory. */
export const ROOT = resolve(process.env.TEPATI_ROOT ?? join(import.meta.dir, "..", ".."));

export const paths = {
  root: ROOT,
  rootEnv: join(ROOT, ".env"),
  envDir: join(ROOT, "deploy", "env"),
  envTemplate: join(ROOT, "deploy", "env", ".env.example"),
  devEnvTemplate: join(ROOT, "deploy", "env", ".env.dev.example"),
  devEnv: join(ROOT, "deploy", "env", ".env.dev"),
  composeBase: join(ROOT, "deploy", "compose.yaml"),
  composeDev: join(ROOT, "deploy", "compose.dev.yaml"),
};
