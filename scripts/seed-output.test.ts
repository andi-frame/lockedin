import { describe, expect, test } from "bun:test";
import { parseSeedLogin } from "./lib/seed-output.ts";

const out = `pact     0199-abc  (active)
backer   today-x-backer@tepati.test  password tepati-seed-1234
doer     today-x-doer@tepati.test  password tepati-seed-1234
inspect  tepatictl pact show 0199-abc
`;

describe("parseSeedLogin", () => {
  test("reads each role's email and password", () => {
    expect(parseSeedLogin(out, "doer")).toEqual({ email: "today-x-doer@tepati.test", password: "tepati-seed-1234" });
    expect(parseSeedLogin(out, "backer")).toEqual({ email: "today-x-backer@tepati.test", password: "tepati-seed-1234" });
  });
  test("is undefined when the line is missing", () => {
    expect(parseSeedLogin("pact  x\n", "doer")).toBeUndefined();
  });
});
