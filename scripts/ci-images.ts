// `bun scripts/ci-images.ts pairs|pull` (used by .github/workflows, see scripts/lib/ciimages.ts).
//   pairs  print "<source> <mirror>" for every image in deploy/ci-images.txt
//   pull   pull each mirror from GHCR and tag it with the source's name, so compose and docker
//          build find it locally; an image that cannot be pulled is reported and left for Docker Hub
import { join } from "node:path";
import { parseImageList, pullPlan } from "./lib/ciimages.ts";
import { paths } from "./lib/paths.ts";

const repo = process.env.GITHUB_REPOSITORY;
if (!repo) {
  console.error("GITHUB_REPOSITORY is not set: this runs in GitHub Actions");
  process.exit(1);
}
const plan = pullPlan(parseImageList(await Bun.file(join(paths.root, "deploy", "ci-images.txt")).text()), repo);
const cmd = process.argv[2];

async function docker(...args: string[]): Promise<boolean> {
  const proc = Bun.spawn(["docker", ...args], { stdout: "inherit", stderr: "inherit" });
  return (await proc.exited) === 0;
}

if (cmd === "pairs") {
  for (const { source, mirror } of plan) console.log(`${source} ${mirror}`);
} else if (cmd === "pull") {
  let missing = 0;
  for (const { source, mirror } of plan) {
    if ((await docker("pull", mirror)) && (await docker("tag", mirror, source))) console.log(`using ${mirror} as ${source}`);
    else {
      missing++;
      console.log(`::warning::${mirror} could not be pulled; ${source} will come from Docker Hub`);
    }
  }
  console.log(missing === 0 ? "every CI image comes from GHCR" : `${missing} of ${plan.length} CI images will come from Docker Hub`);
} else {
  console.error("usage: bun scripts/ci-images.ts pairs|pull");
  process.exit(1);
}
