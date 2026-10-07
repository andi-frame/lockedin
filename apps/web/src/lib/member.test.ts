import { describe, expect, test } from "bun:test";
import { memberSlot } from "./member";

describe("memberSlot", () => {
  test("two members of a pact get different slots", () => {
    expect(memberSlot("a", ["a", "b"])).not.toBe(memberSlot("b", ["a", "b"]));
  });
  test("does not depend on the order the API lists the members in", () => {
    expect(memberSlot("b", ["a", "b"])).toBe(memberSlot("b", ["b", "a"]));
  });
  test("the same member keeps one slot for the whole pact", () => {
    const ids = ["u-2", "u-1"];
    expect(memberSlot("u-1", ids)).toBe(0);
    expect(memberSlot("u-2", ids)).toBe(1);
  });
  test("an id that is not in the pact falls back to slot 0", () => {
    expect(memberSlot("stranger", ["a", "b"])).toBe(0);
  });
});
