import { describe, expect, test } from "bun:test";
import { pollDelay, putHeaders, targetSize } from "./net";

describe("putHeaders", () => {
  test("drops Content-Length, which the browser sets itself, in any case", () => {
    expect(putHeaders({ "Content-Length": "123", "Content-Type": "image/webp", "x-amz-meta": "a" })).toEqual({
      "Content-Type": "image/webp",
      "x-amz-meta": "a",
    });
    expect(putHeaders({ "content-length": "1", Host: "x" })).toEqual({ Host: "x" });
  });
});

describe("pollDelay", () => {
  test("starts at one second and backs off to five", () => {
    expect([0, 1, 2, 3, 4, 5, 9].map(pollDelay)).toEqual([1000, 1500, 2250, 3375, 5000, 5000, 5000]);
  });
});

describe("targetSize", () => {
  test("keeps the aspect ratio and the longest side at the limit", () => {
    expect(targetSize(4000, 3000, 2048)).toEqual({ width: 2048, height: 1536 });
    expect(targetSize(3000, 4000, 2048)).toEqual({ width: 1536, height: 2048 });
  });
  test("never enlarges", () => {
    expect(targetSize(800, 600, 2048)).toEqual({ width: 800, height: 600 });
  });
  test("never produces a zero side", () => {
    expect(targetSize(10000, 1, 2048)).toEqual({ width: 2048, height: 1 });
  });
});
