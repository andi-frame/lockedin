import { describe, expect, test } from "bun:test";
import { REASON_MAX, REASON_MIN, reasonState } from "./reason";

describe("reasonState", () => {
  test("counts characters after trimming, like the server's minimum of 10", () => {
    expect(reasonState("         ")).toMatchObject({ ok: false, length: 0, missing: REASON_MIN });
    expect(reasonState("  pendek  ")).toMatchObject({ ok: false, length: 6, missing: 4 });
    expect(reasonState("tepat sepuluh")).toMatchObject({ ok: true, missing: 0 });
    expect(reasonState("1234567890").ok).toBe(true);
    expect(reasonState("123456789").ok).toBe(false);
  });

  test("counts characters, not bytes", () => {
    // Ten Indonesian-friendly characters, some of them multi-byte.
    expect(reasonState("éééééééééé").ok).toBe(true);
    // An emoji is one character even though it is two UTF-16 units.
    expect(reasonState("😀😀😀😀😀😀😀😀😀").ok).toBe(false);
    expect(reasonState("😀😀😀😀😀😀😀😀😀😀").ok).toBe(true);
  });

  test("refuses more than the server's maximum", () => {
    expect(reasonState("a".repeat(REASON_MAX)).ok).toBe(true);
    expect(reasonState("a".repeat(REASON_MAX + 1))).toMatchObject({ ok: false, tooLong: true });
  });

  test("returns the trimmed text, which is what is sent", () => {
    expect(reasonState("  alasan yang cukup  ").trimmed).toBe("alasan yang cukup");
  });
});
