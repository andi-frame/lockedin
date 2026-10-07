import { describe, expect, test } from "bun:test";
import { isActive, navItems } from "./nav";

describe("navItems", () => {
  test("are the four destinations in the order the rail and the tab bar show them", () => {
    expect(navItems.map((i) => i.key)).toEqual(["today", "pacts", "review", "settings"]);
    expect(navItems.map((i) => i.href)).toEqual(["/today", "/pacts", "/review", "/settings"]);
  });
});

describe("isActive", () => {
  test.each([
    ["/today", "/today", true],
    ["/pacts", "/pacts", true],
    ["/pacts/9f2", "/pacts", true],
    ["/pacts/new", "/pacts", true],
    ["/pactsx", "/pacts", false],
    ["/review", "/pacts", false],
    ["/today", "/settings", false],
  ])("%s under %s is %p", (pathname, href, expected) => {
    expect(isActive(pathname, href)).toBe(expected);
  });
  test("a query string or trailing slash does not matter", () => {
    expect(isActive("/review/", "/review")).toBe(true);
  });
});
