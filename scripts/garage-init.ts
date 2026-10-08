// `bun run garage:init`: idempotent Garage bootstrap (RUNNING.md §5).
// Every step checks current state first, so a second run only prints no-ops.
import { PutBucketCorsCommand, S3Client } from "@aws-sdk/client-s3";
import { randomBytes } from "node:crypto";
import { readEnvFile, setEnvValue } from "./lib/env.ts";
import { composeBase } from "./lib/compose.ts";
import { paths } from "./lib/paths.ts";
import { capture, fail, log } from "./lib/proc.ts";

const KEY_NAME = "tepati-app";

/**
 * Which Garage to set up. The default is the dev infra (root .env, S3 reachable from the host, so
 * the CORS rule is set from here). A deploy passes its own compose project and env file; its S3
 * port is not published, so `deploy:up` sets the CORS rule from inside the network afterwards
 * (`tepatictl storage-init`).
 */
export interface GarageTarget {
  compose: () => Promise<string[]>;
  envFile: string;
  /** Env files that receive generated S3 credentials. */
  persistTo: string[];
  hostCors: boolean;
  /** Buckets beyond staging and media (the backup bucket, when backups run). */
  extraBuckets?: string[];
}

export const devTarget = (): GarageTarget => ({ compose: composeBase, envFile: paths.rootEnv, persistTo: [paths.rootEnv, paths.devEnv], hostCors: true });

async function garage(target: GarageTarget, args: string[]) {
  return capture([...(await target.compose()), "exec", "-T", "garage", "/garage", ...args]);
}

async function garageOk(target: GarageTarget, args: string[]): Promise<string> {
  const r = await garage(target, args);
  if (r.code !== 0) fail(`garage ${args.join(" ")} failed:\n${r.stderr || r.stdout}`);
  return r.stdout;
}

/** Writes a key into the env files that exist (root .env and docker dev env, or the deploy's env file). */
async function persistEnv(files: string[], values: Record<string, string>) {
  for (const file of files) {
    const f = Bun.file(file);
    if (!(await f.exists())) continue;
    let text = await f.text();
    for (const [k, v] of Object.entries(values)) text = setEnvValue(text, k, v);
    await Bun.write(file, text);
  }
}

export async function garageInit(target: GarageTarget = devTarget()): Promise<void> {
  const env = await readEnvFile(target.envFile);

  // 1. Layout: a fresh node has layout version 0 and no role.
  const nodeId = (await garageOk(target, ["node", "id", "-q"])).trim().split("@")[0];
  if (!nodeId) fail("could not read the Garage node id");
  const layout = await garageOk(target, ["layout", "show"]);
  const version = Number(/layout version:\s*(\d+)/i.exec(layout)?.[1] ?? "0");
  if (version === 0) {
    await garageOk(target, ["layout", "assign", "-z", "dc1", "-c", env.get("GARAGE_CAPACITY") || "10G", nodeId]);
    await garageOk(target, ["layout", "apply", "--version", "1"]);
    log.ok(`layout assigned to node ${nodeId.slice(0, 16)}…`);
  } else {
    log.skip(`layout already at version ${version}`);
  }

  // 2. Access key: generated locally once, then imported, so `infra:reset` keeps .env valid.
  let accessKey = env.get("S3_ACCESS_KEY") ?? "";
  let secretKey = env.get("S3_SECRET_KEY") ?? "";
  if (!accessKey || !secretKey) {
    accessKey = `GK${randomBytes(12).toString("hex")}`;
    secretKey = randomBytes(32).toString("hex");
    await persistEnv(target.persistTo, { S3_ACCESS_KEY: accessKey, S3_SECRET_KEY: secretKey });
    log.ok("generated S3 credentials into .env");
  }
  if ((await garage(target, ["key", "info", accessKey])).code === 0) {
    log.skip(`key ${KEY_NAME} exists`);
  } else {
    await garageOk(target, ["key", "import", "--yes", "-n", KEY_NAME, accessKey, secretKey]);
    log.ok(`key ${KEY_NAME} imported`);
  }

  // 3. Buckets and permissions (`bucket allow` is idempotent).
  const buckets = [env.get("S3_BUCKET_STAGING") || "tepati-staging", env.get("S3_BUCKET_MEDIA") || "tepati-media", ...(target.extraBuckets ?? [])];
  for (const bucket of buckets) {
    if ((await garage(target, ["bucket", "info", bucket])).code === 0) log.skip(`bucket ${bucket} exists`);
    else {
      await garageOk(target, ["bucket", "create", bucket]);
      log.ok(`bucket ${bucket} created`);
    }
    await garageOk(target, ["bucket", "allow", "--read", "--write", "--owner", bucket, "--key", accessKey]);
  }

  // 4. CORS on the staging bucket so the browser can PUT presigned uploads (ADR-0005).
  if (!target.hostCors) {
    log.skip("CORS is set from inside the network after the app is up (tepatictl storage-init)");
    return;
  }
  const s3 = new S3Client({
    endpoint: env.get("S3_ENDPOINT") || "http://localhost:3900",
    region: env.get("S3_REGION") || "garage",
    forcePathStyle: true,
    credentials: { accessKeyId: accessKey, secretAccessKey: secretKey },
  });
  const origin = env.get("APP_BASE_URL") || "http://localhost:3000";
  await s3.send(
    new PutBucketCorsCommand({
      Bucket: buckets[0],
      CORSConfiguration: {
        CORSRules: [
          {
            AllowedOrigins: [origin],
            AllowedMethods: ["PUT", "GET", "HEAD"],
            AllowedHeaders: ["*"],
            ExposeHeaders: ["ETag"],
            MaxAgeSeconds: 3600,
          },
        ],
      },
    }),
  );
  log.ok(`CORS on ${buckets[0]} allows PUT from ${origin}`);
}

if (import.meta.main) await garageInit();
