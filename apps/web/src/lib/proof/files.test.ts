import { describe, expect, test } from "bun:test";
import { MAX_ATTACHMENTS, checkFile, declaredMime, roomLeft } from "./files";

const MB = 1024 * 1024;
const f = (name: string, type: string, size: number) => ({ name, type, size });

// The client only pre-checks (SPEC §8); the server sniffs the real bytes and decides.
describe("checkFile", () => {
  test.each([
    ["foto.jpg", "image/jpeg", "image"],
    ["foto.png", "image/png", "image"],
    ["foto.webp", "image/webp", "image"],
    ["foto.heic", "image/heic", "image"],
    ["foto.heif", "image/heif", "image"],
    ["foto.avif", "image/avif", "image"],
    ["anim.gif", "image/gif", "image"],
    ["klip.mp4", "video/mp4", "video"],
    ["klip.mov", "video/quicktime", "video"],
    ["klip.webm", "video/webm", "video"],
    ["catatan.pdf", "application/pdf", "file"],
  ])("%s (%s) is a %s", (name, type, kind) => {
    expect(checkFile(f(name, type, MB))).toEqual({ ok: true, kind: kind as "image" | "video" | "file" });
  });

  test("a missing type falls back to the extension, as some phones report HEIC with no type", () => {
    expect(checkFile(f("IMG_0001.HEIC", "", MB))).toEqual({ ok: true, kind: "image" });
    expect(checkFile(f("clip.MOV", "", MB))).toEqual({ ok: true, kind: "video" });
  });

  test.each([
    ["a text file", f("a.txt", "text/plain", 10)],
    ["a zip", f("a.zip", "application/zip", 10)],
    ["an svg", f("a.svg", "image/svg+xml", 10)],
    ["an unknown type with no extension", f("data", "", 10)],
  ])("refuses %s as unsupported", (_n, file) => {
    expect(checkFile(file)).toEqual({ ok: false, reason: "unsupported" });
  });

  test("refuses an empty file", () => {
    expect(checkFile(f("a.png", "image/png", 0))).toEqual({ ok: false, reason: "empty" });
  });

  test.each([
    ["an image over 15 MB", f("a.png", "image/png", 15 * MB + 1), 15],
    ["a video over 200 MB", f("a.mp4", "video/mp4", 250 * MB), 200],
    ["a pdf over 20 MB", f("a.pdf", "application/pdf", 20 * MB + 1), 20],
  ])("refuses %s and names the limit", (_n, file, limit) => {
    expect(checkFile(file)).toEqual({ ok: false, reason: "too_large", limitMb: limit });
  });

  test("a file exactly at the limit passes", () => {
    expect(checkFile(f("a.pdf", "application/pdf", 20 * MB))).toEqual({ ok: true, kind: "file" });
  });
});

describe("declaredMime", () => {
  test("is the browser's type when it has one", () => {
    expect(declaredMime(f("a.png", "image/png", 1))).toBe("image/png");
  });
  test("is inferred from the extension when the type is empty", () => {
    expect(declaredMime(f("IMG_1.HEIC", "", 1))).toBe("image/heic");
    expect(declaredMime(f("clip.MOV", "", 1))).toBe("video/quicktime");
    expect(declaredMime(f("a.pdf", "", 1))).toBe("application/pdf");
  });
});

describe("roomLeft", () => {
  test("at most ten attachments per proof", () => {
    expect(MAX_ATTACHMENTS).toBe(10);
    expect(roomLeft(0)).toBe(10);
    expect(roomLeft(7)).toBe(3);
    expect(roomLeft(10)).toBe(0);
    expect(roomLeft(12)).toBe(0);
  });
});
