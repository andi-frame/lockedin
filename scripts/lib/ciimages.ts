// CI pulls its base images from Docker Hub, which limits anonymous pulls per IP while GitHub's runners
// share IPs (PLAN 9.8). Official images (`postgres`, `redis`, `golang`) come through Google's mirror,
// configured in ci.yml. The rest, from a Docker Hub organisation, are copied to this repository's own
// GHCR packages by .github/workflows/mirror-images.yml, and the CI jobs pull them from there and tag
// them with their original names, so nothing in compose or the Dockerfiles changes. Production and
// laptops keep using Docker Hub.

/** One image per line; `#` starts a comment. */
export function parseImageList(text: string): string[] {
  return text
    .split("\n")
    .map((line) => line.replace(/#.*/, "").trim())
    .filter(Boolean);
}

/** The mirror's name: lowercase, under the repository's packages, with `/` in the source turned into `-`. */
export function mirrorName(source: string, repo: string): string {
  const [name = "", tag] = source.split(/:(.*)/s, 2);
  if (!tag) throw new Error(`"${source}" has no tag: a mirror of a floating default would be a moving target`);
  return `ghcr.io/${repo}/ci-${name.replaceAll("/", "-")}:${tag}`.toLowerCase();
}

export type Pull = { source: string; mirror: string };

export const pullPlan = (images: string[], repo: string): Pull[] => images.map((source) => ({ source, mirror: mirrorName(source, repo) }));

/**
 * The images in compose files and Dockerfiles that are neither official (no `/`), nor ours
 * (`tepati-*`), nor templated, nor on another registry (a dotted first segment): the ones Google's
 * mirror does not carry. Sorted, without duplicates.
 */
export function externalImages(texts: string[]): string[] {
  const found = new Set<string>();
  for (const text of texts) {
    for (const line of text.split("\n")) {
      const compose = /^\s*image:\s*["']?([^\s"']+)/.exec(line)?.[1];
      const dockerfile = /^\s*FROM\s+(?:--\S+\s+)*([^\s]+)/i.exec(line)?.[1];
      const image = compose ?? dockerfile;
      if (!image || image.includes("${") || image.startsWith("tepati-")) continue;
      const first = image.split("/")[0] ?? "";
      if (!image.includes("/") || first.includes(".") || first.includes(":")) continue;
      found.add(image);
    }
  }
  return [...found].sort();
}
