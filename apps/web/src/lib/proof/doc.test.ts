import { describe, expect, test } from "bun:test";
import { analyzeDoc, allowedMarks, allowedNodes, type ProofDoc } from "./doc";

const p = (...content: ProofDoc[]): ProofDoc => ({ type: "paragraph", content });
const t = (text: string, marks?: ProofDoc["marks"]): ProofDoc => ({ type: "text", text, ...(marks ? { marks } : {}) });
const doc = (...content: ProofDoc[]): ProofDoc => ({ type: "doc", content });

// These cases mirror internal/domain/proofdoc.go (ParseProofDoc): the word count that gates the
// submit button must be the one the server will compute, or a button that says "ready" gets a 422.
describe("analyzeDoc", () => {
  test("counts words across blocks, which separate words", () => {
    const r = analyzeDoc(doc(p(t("Sesi 1")), p(t("Mengerjakan 50 soal TPS")), p(t("benar 41"))));
    expect(r).toMatchObject({ ok: true, words: 8 });
  });

  test("a hard break separates words too", () => {
    expect(analyzeDoc(doc(p(t("a"), { type: "hardBreak" }, t("b"))))).toMatchObject({ ok: true, words: 2 });
  });

  test("list items and headings separate words", () => {
    const r = analyzeDoc(
      doc(
        { type: "heading", attrs: { level: 2 }, content: [t("Judul")] },
        { type: "bulletList", content: [{ type: "listItem", content: [p(t("satu"))] }, { type: "listItem", content: [p(t("dua"))] }] },
      ),
    );
    expect(r).toMatchObject({ ok: true, words: 3 });
  });

  test("an empty document has no words", () => {
    expect(analyzeDoc(doc(p()))).toMatchObject({ ok: true, words: 0, text: "" });
  });

  test("any Unicode whitespace separates words", () => {
    expect(analyzeDoc(doc(p(t("a b\tc"))))).toMatchObject({ ok: true, words: 3 });
  });

  test.each<[string, ProofDoc]>([
    ["an unknown node", doc({ type: "table" })],
    ["a heading level 1", doc({ type: "heading", attrs: { level: 1 }, content: [t("x")] })],
    ["a heading level 4", doc({ type: "heading", attrs: { level: 4 }, content: [t("x")] })],
    ["an unknown mark", doc(p(t("x", [{ type: "underline" }])))],
    ["a javascript: link", doc(p(t("x", [{ type: "link", attrs: { href: "javascript:alert(1)" } }])))],
    ["a link with no host", doc(p(t("x", [{ type: "link", attrs: { href: "https://" } }])))],
    ["a link with no href", doc(p(t("x", [{ type: "link" }])))],
    ["an image with an external src", doc({ type: "attachmentImage", attrs: { attachmentId: "11111111-1111-4111-8111-111111111111", src: "https://x.test/a.png" } })],
    ["an image with a bad id", doc({ type: "attachmentImage", attrs: { attachmentId: "nope" } })],
    ["a root that is not doc", p(t("x"))],
  ])("rejects %s", (_name, d) => {
    expect(analyzeDoc(d)).toMatchObject({ ok: false });
  });

  test("accepts http and https links and every allowed mark", () => {
    const marks = [
      { type: "bold" },
      { type: "italic" },
      { type: "strike" },
      { type: "code" },
      { type: "link", attrs: { href: "https://example.com/a?b=1" } },
    ];
    expect(analyzeDoc(doc(p(t("x", marks))))).toMatchObject({ ok: true });
    expect(analyzeDoc(doc(p(t("x", [{ type: "link", attrs: { href: "http://example.com" } }]))))).toMatchObject({ ok: true });
  });

  test("an inline image only references an attachment, and its id is reported", () => {
    const id = "11111111-1111-4111-8111-111111111111";
    expect(analyzeDoc(doc({ type: "attachmentImage", attrs: { attachmentId: id } }))).toMatchObject({ ok: true, attachmentIds: [id] });
  });

  test("nesting deeper than 12 is refused", () => {
    let node: ProofDoc = p(t("deep"));
    for (let i = 0; i < 12; i++) node = { type: "blockquote", content: [node] };
    expect(analyzeDoc(doc(node))).toMatchObject({ ok: false });
    let shallow: ProofDoc = p(t("ok"));
    for (let i = 0; i < 8; i++) shallow = { type: "blockquote", content: [shallow] };
    expect(analyzeDoc(doc(shallow))).toMatchObject({ ok: true });
  });

  test("a document over 200 KB is refused", () => {
    expect(analyzeDoc(doc(p(t("x".repeat(201 * 1024)))))).toMatchObject({ ok: false });
  });
});

test("the allow-list is the server's (SPEC §8)", () => {
  expect([...allowedNodes].sort()).toEqual(
    ["attachmentImage", "blockquote", "bulletList", "codeBlock", "doc", "hardBreak", "heading", "horizontalRule", "listItem", "orderedList", "paragraph", "taskItem", "taskList", "text"].sort(),
  );
  expect([...allowedMarks].sort()).toEqual(["bold", "code", "italic", "link", "strike"]);
});
