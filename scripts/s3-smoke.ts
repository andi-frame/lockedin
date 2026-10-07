// `bun run s3:smoke`: proves the app credentials can reach both buckets.
import { DeleteObjectCommand, GetObjectCommand, ListBucketsCommand, PutObjectCommand, S3Client } from "@aws-sdk/client-s3";
import { readEnvFile } from "./lib/env.ts";
import { paths } from "./lib/paths.ts";
import { fail, log } from "./lib/proc.ts";

const env = await readEnvFile(paths.rootEnv);
const s3 = new S3Client({
  endpoint: env.get("S3_ENDPOINT") || "http://localhost:3900",
  region: env.get("S3_REGION") || "garage",
  forcePathStyle: true,
  credentials: { accessKeyId: env.get("S3_ACCESS_KEY") ?? "", secretAccessKey: env.get("S3_SECRET_KEY") ?? "" },
});

const expected = [env.get("S3_BUCKET_STAGING") || "tepati-staging", env.get("S3_BUCKET_MEDIA") || "tepati-media"];
const { Buckets = [] } = await s3.send(new ListBucketsCommand({}));
const names = Buckets.map((b) => b.Name);
for (const b of expected) if (!names.includes(b)) fail(`bucket ${b} not visible to the app key (got: ${names.join(", ")})`);
log.ok(`buckets: ${names.join(", ")}`);

for (const bucket of expected) {
  const Key = `_smoke/${crypto.randomUUID()}.txt`;
  await s3.send(new PutObjectCommand({ Bucket: bucket, Key, Body: "tepati" }));
  const got = await (await s3.send(new GetObjectCommand({ Bucket: bucket, Key }))).Body?.transformToString();
  await s3.send(new DeleteObjectCommand({ Bucket: bucket, Key }));
  if (got !== "tepati") fail(`round-trip on ${bucket} returned ${JSON.stringify(got)}`);
  log.ok(`put/get/delete on ${bucket}`);
}
