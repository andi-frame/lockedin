import { describe, expect, test } from "bun:test";
import { checkEvidence } from "./rules";

// Mirrors `domain.Evidence.Check`: attachments still processing never count and block the submit.
describe("checkEvidence", () => {
  const rules = { minAttachments: 1, minWords: 20 };

  test("is satisfied when words and ready attachments reach the minimums", () => {
    expect(checkEvidence(rules, { words: 20, ready: 1, pending: 0 })).toEqual({ ok: true, needWords: 0, needFiles: 0, waiting: false });
  });
  test("says how many words are missing", () => {
    expect(checkEvidence(rules, { words: 12, ready: 1, pending: 0 })).toMatchObject({ ok: false, needWords: 8, needFiles: 0 });
  });
  test("says how many attachments are missing", () => {
    expect(checkEvidence({ minAttachments: 3, minWords: 0 }, { words: 0, ready: 1, pending: 0 })).toMatchObject({ ok: false, needFiles: 2 });
  });
  test("an attachment still being processed blocks the submit and does not count", () => {
    expect(checkEvidence(rules, { words: 30, ready: 0, pending: 1 })).toEqual({ ok: false, needWords: 0, needFiles: 1, waiting: true });
    expect(checkEvidence({ minAttachments: 0, minWords: 0 }, { words: 0, ready: 0, pending: 1 })).toMatchObject({ ok: false, waiting: true });
  });
  test("no rules means anything goes, even an empty proof", () => {
    expect(checkEvidence({ minAttachments: 0, minWords: 0 }, { words: 0, ready: 0, pending: 0 }).ok).toBe(true);
  });
});
