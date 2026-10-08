// A client-side mirror of `domain.ParseProofDoc` (apps/server/internal/domain/proofdoc.go). The
// server stays the authority and re-validates; this exists so the submit button's idea of "enough
// words" is the server's own count, and so a document the server would refuse is never sent.

export type ProofDoc = {
  type: string;
  attrs?: Record<string, unknown>;
  content?: ProofDoc[];
  text?: string;
  marks?: { type: string; attrs?: Record<string, unknown> }[];
};

/** SPEC §8. Keep in step with `allowedNodes` and `allowedMarks` in proofdoc.go. */
export const allowedNodes: ReadonlySet<string> = new Set([
  "doc", "paragraph", "heading", "bulletList", "orderedList", "listItem", "blockquote", "codeBlock",
  "hardBreak", "horizontalRule", "text", "taskList", "taskItem", "attachmentImage",
]);
export const allowedMarks: ReadonlySet<string> = new Set(["bold", "italic", "strike", "code", "link"]);

const MAX_BYTES = 200 << 10;
const MAX_DEPTH = 12;
const blockEnds: ReadonlySet<string> = new Set(["paragraph", "heading", "listItem", "taskItem", "codeBlock", "blockquote", "hardBreak"]);
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export type DocAnalysis = { ok: true; text: string; words: number; attachmentIds: string[] } | { ok: false; reason: string };

function safeLink(href: unknown): boolean {
  try {
    const u = new URL(String(href));
    return (u.protocol === "http:" || u.protocol === "https:") && u.host !== "";
  } catch {
    return false;
  }
}

export function analyzeDoc(root: ProofDoc): DocAnalysis {
  if (new TextEncoder().encode(JSON.stringify(root)).length > MAX_BYTES) return { ok: false, reason: "size" };
  if (root.type !== "doc") return { ok: false, reason: "root" };
  let text = "";
  const ids: string[] = [];

  function walk(n: ProofDoc, depth: number): string | null {
    if (depth > MAX_DEPTH) return "depth";
    if (!allowedNodes.has(n.type)) return `node ${n.type}`;
    if (n.type === "heading") {
      const level = n.attrs?.level;
      if (level !== 2 && level !== 3) return "heading level";
    } else if (n.type === "attachmentImage") {
      const id = String(n.attrs?.attachmentId);
      if (!uuid.test(id) || Object.keys(n.attrs ?? {}).length !== 1) return "attachmentImage";
      ids.push(id);
    } else if (n.type === "text") {
      text += n.text ?? "";
    }
    for (const m of n.marks ?? []) {
      if (!allowedMarks.has(m.type)) return `mark ${m.type}`;
      if (m.type === "link" && !safeLink(m.attrs?.href)) return "link";
    }
    for (const c of n.content ?? []) {
      const bad = walk(c, depth + 1);
      if (bad) return bad;
    }
    if (blockEnds.has(n.type)) text += "\n";
    return null;
  }

  const bad = walk(root, 0);
  if (bad) return { ok: false, reason: bad };
  const trimmed = text.trim();
  // Go's strings.Fields splits on Unicode white space, as the \s class does here.
  const words = trimmed === "" ? 0 : trimmed.split(/\s+/).length;
  return { ok: true, text: trimmed, words, attachmentIds: ids };
}

/**
 * What a person typed into the link box, as an http(s) URL the server will accept, or null.
 * A bare address gets https://; every other scheme (javascript:, data:, mailto:) is refused.
 */
export function normalizeLink(raw: string): string | null {
  const s = raw.trim();
  if (!s || /\s/.test(s)) return null;
  const hasHttp = /^https?:\/\//i.test(s);
  // "name:" that is not "host:8080" is some other scheme (javascript:, data:, mailto:, ftp:).
  if (!hasHttp && /^[a-z][a-z0-9+.-]*:(?!\d)/i.test(s)) return null;
  try {
    const u = new URL(hasHttp ? s : `https://${s}`);
    return (u.protocol === "http:" || u.protocol === "https:") && u.host !== "" && u.hostname.includes(".") ? u.toString() : null;
  } catch {
    return null;
  }
}
