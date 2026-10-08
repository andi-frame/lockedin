import { expect, test } from "bun:test";
import { forwardedHeaders } from "./forward";

// The API limits requests per client IP. The web server calls it on the visitor's behalf, so
// without the visitor's address every visitor would share the web container's budget.
test("the visitor's address is passed on, so the API limits each visitor on their own", () => {
  const incoming = new Headers({ "x-forwarded-for": "203.0.113.7", "user-agent": "x" });
  expect(forwardedHeaders(incoming)).toEqual({ "x-forwarded-for": "203.0.113.7" });
});

test("a chain of proxies is passed on whole", () => {
  expect(forwardedHeaders(new Headers({ "x-forwarded-for": "203.0.113.7, 10.0.0.2" }))).toEqual({ "x-forwarded-for": "203.0.113.7, 10.0.0.2" });
});

test("nothing is invented when there is no address (dev, health checks)", () => {
  expect(forwardedHeaders(new Headers())).toEqual({});
});

test("no other incoming header is copied", () => {
  const incoming = new Headers({ cookie: "a=b", host: "evil", "x-csrf-token": "t", "x-forwarded-host": "evil" });
  expect(forwardedHeaders(incoming)).toEqual({});
});
