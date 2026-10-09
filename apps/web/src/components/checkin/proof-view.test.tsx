import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import type { ProofDoc } from "@/lib/proof/doc";
import { ProofView } from "./proof-view";

const doc = (href: string): ProofDoc => ({
  type: "doc",
  content: [{ type: "paragraph", content: [{ type: "text", text: "catatan", marks: [{ type: "link", attrs: { href } }] }] }],
});

// A proof is written by the other person, so its links are untrusted: they must not hand over the
// opener or the referrer.
test("a link in a proof opens in a new tab without the opener or referrer", () => {
  const html = renderToStaticMarkup(<ProofView doc={doc("https://example.com/soal")} />);
  expect(html).toContain('href="https://example.com/soal"');
  expect(html).toContain('target="_blank"');
  expect(html).toMatch(/rel="[^"]*noopener[^"]*"/);
  expect(html).toMatch(/rel="[^"]*noreferrer[^"]*"/);
});

test("a javascript: link is never drawn as a link", () => {
  const html = renderToStaticMarkup(<ProofView doc={doc("javascript:alert(1)")} />);
  expect(html).not.toContain("javascript:");
});
