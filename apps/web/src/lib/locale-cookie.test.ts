import { expect, test } from "bun:test";
import { localeCookieString } from "./locale-cookie";

test("the cookie the pages read is named, scoped to the whole site and lasts a year", () => {
  expect(localeCookieString("en")).toBe("tepati_locale=en; path=/; max-age=31536000; samesite=lax");
  expect(localeCookieString("id")).toBe("tepati_locale=id; path=/; max-age=31536000; samesite=lax");
});
