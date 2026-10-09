import { expect, test } from "bun:test";
import { join } from "node:path";
import { externalImages, mirrorName, parseImageList, pullPlan } from "./ciimages.ts";
import { paths } from "./paths.ts";

test("the list ignores blank lines and comments, and trims", () => {
  expect(parseImageList("# why\n\naxllent/mailpit:v1.27  \n  oven/bun:1.4 # base\n")).toEqual(["axllent/mailpit:v1.27", "oven/bun:1.4"]);
});

test("a mirror lives under the repository's own packages, named after the source", () => {
  expect(mirrorName("axllent/mailpit:v1.27", "andi-frame/lockedin")).toBe("ghcr.io/andi-frame/lockedin/ci-axllent-mailpit:v1.27");
  expect(mirrorName("oven/bun:1.4-slim", "andi-frame/lockedin")).toBe("ghcr.io/andi-frame/lockedin/ci-oven-bun:1.4-slim");
  // Registry names are lowercase.
  expect(mirrorName("Org/Img:1", "Andi-Frame/LockedIn")).toBe("ghcr.io/andi-frame/lockedin/ci-org-img:1");
});

test("an image without a tag is refused: a floating default would make the mirror a moving target", () => {
  expect(() => mirrorName("axllent/mailpit", "a/b")).toThrow(/tag/);
});

test("the pull plan names each source and its mirror", () => {
  expect(pullPlan(["dxflrs/garage:v2.4.1"], "andi-frame/lockedin")).toEqual([
    { source: "dxflrs/garage:v2.4.1", mirror: "ghcr.io/andi-frame/lockedin/ci-dxflrs-garage:v2.4.1" },
  ]);
});

test("only images from a Docker Hub organisation are external: official ones come through Google's mirror, ours are built", () => {
  const compose = `
services:
  a:
    image: axllent/mailpit:v1.27
  b:
    image: postgres:18
  c:
    image: tepati-server:\${TEPATI_ENV:-dev}
  d:
    image: "oven/bun:1.4"
`;
  const dockerfile = "# syntax=docker/dockerfile:1\nFROM golang:1.26-bookworm AS build\nFROM oven/bun:1.4-slim\nFROM build AS final\n";
  expect(externalImages([compose, dockerfile])).toEqual(["axllent/mailpit:v1.27", "oven/bun:1.4", "oven/bun:1.4-slim"]);
});

// The list drifting from what compose and the Dockerfiles use would put the unmirrored image back on
// Docker Hub without anyone noticing, so this fails when an image is added or its tag changes.
test("deploy/ci-images.txt lists exactly the Docker Hub organisation images the repo uses", async () => {
  const read = (f: string) => Bun.file(join(paths.root, f)).text();
  const files = ["deploy/compose.yaml", "deploy/compose.dev.yaml", "deploy/compose.prod.yaml", "deploy/docker/server.Dockerfile", "deploy/docker/web.Dockerfile", "deploy/docker/dev-server.Dockerfile"];
  const used = externalImages(await Promise.all(files.map(read)));
  const listed = parseImageList(await read("deploy/ci-images.txt"));
  expect([...listed].sort()).toEqual([...used].sort());
});
