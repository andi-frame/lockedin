import { describe, expect, test } from "bun:test";
import { guard, isPublicPath, loginRedirect, safeNext } from "./guard";

describe("safeNext", () => {
  test.each([
    ["/today", "/today"],
    ["/pacts/123?tab=ledger", "/pacts/123?tab=ledger"],
    ["/invite/abc_DEF-9", "/invite/abc_DEF-9"],
  ])("keeps the same-site path %s", (input, expected) => {
    expect(safeNext(input)).toBe(expected);
  });

  test.each([
    ["https://evil.example/today"],
    ["//evil.example"],
    ["/\\evil.example"],
    ["javascript:alert(1)"],
    ["today"],
    [""],
    ["/\nSet-Cookie: x=1"],
  ])("rejects %p so a login link cannot send people off site", (input) => {
    expect(safeNext(input)).toBe("/today");
  });

  test("missing and repeated values fall back to /today", () => {
    expect(safeNext(null)).toBe("/today");
    expect(safeNext(undefined)).toBe("/today");
    expect(safeNext(["/pacts", "/review"])).toBe("/today");
  });

  test("never sends people back to the auth pages", () => {
    expect(safeNext("/login")).toBe("/today");
    expect(safeNext("/register?next=/pacts")).toBe("/today");
  });
});

describe("isPublicPath", () => {
  test.each(["/login", "/register", "/login/", "/dev/kitchen-sink"])("%s is public", (p) => {
    expect(isPublicPath(p)).toBe(true);
  });
  test.each(["/", "/today", "/pacts/1", "/review", "/settings", "/invite/abc", "/loginx"])("%s needs a session", (p) => {
    expect(isPublicPath(p)).toBe(false);
  });
});

describe("loginRedirect", () => {
  test("remembers where the visitor was going", () => {
    expect(loginRedirect("/pacts/1", "?tab=ledger")).toBe("/login?next=%2Fpacts%2F1%3Ftab%3Dledger");
  });
  test("the home page does not leave a next to come back to", () => {
    expect(loginRedirect("/", "")).toBe("/login");
  });
});

describe("guard", () => {
  test("a visitor without a session is sent to login from a private page", () => {
    expect(guard({ pathname: "/today", search: "", hasSession: false })).toEqual({ redirect: "/login?next=%2Ftoday" });
  });
  test("an invite link keeps its token through login", () => {
    expect(guard({ pathname: "/invite/tok", search: "", hasSession: false })).toEqual({
      redirect: "/login?next=%2Finvite%2Ftok",
    });
  });
  test("the login page stays open without a session", () => {
    expect(guard({ pathname: "/login", search: "", hasSession: false })).toBeNull();
  });
  test("a session cookie lets a private page through", () => {
    expect(guard({ pathname: "/today", search: "", hasSession: true })).toBeNull();
  });
  // The cookie can be stale (the server expired the session), so the proxy cannot tell. The login
  // page asks the API itself; if the proxy also bounced /login to /today there would be a loop.
  test("a session cookie does not bounce the login page", () => {
    expect(guard({ pathname: "/login", search: "", hasSession: true })).toBeNull();
  });
  test("the home page goes to /today with a session and /login without", () => {
    expect(guard({ pathname: "/", search: "", hasSession: true })).toEqual({ redirect: "/today" });
    expect(guard({ pathname: "/", search: "", hasSession: false })).toEqual({ redirect: "/login" });
  });
});
