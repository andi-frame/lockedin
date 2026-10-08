import { getSchema } from "@tiptap/core";
import { describe, expect, test } from "bun:test";
import { allowedMarks, allowedNodes } from "./doc";
import { proofExtensions } from "./extensions";

describe("the editor only offers what the server accepts", () => {
  const schema = getSchema(proofExtensions());

  test("every node type is on the server's allow-list", () => {
    const extra = Object.keys(schema.nodes).filter((n) => !allowedNodes.has(n));
    expect(extra).toEqual([]);
  });

  test("every mark type is on the server's allow-list", () => {
    const extra = Object.keys(schema.marks).filter((m) => !allowedMarks.has(m));
    expect(extra).toEqual([]);
  });

  test("the formatting people expect is there", () => {
    for (const n of ["paragraph", "heading", "bulletList", "orderedList", "listItem", "blockquote", "codeBlock", "hardBreak", "horizontalRule", "taskList", "taskItem"]) {
      expect(schema.nodes[n]).toBeDefined();
    }
    for (const m of ["bold", "italic", "strike", "code", "link"]) expect(schema.marks[m]).toBeDefined();
  });
});
