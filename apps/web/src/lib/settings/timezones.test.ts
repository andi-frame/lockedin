import { describe, expect, test } from "bun:test";
import { timezoneChoices } from "./timezones";

describe("timezoneChoices", () => {
  test("lists the browser's zones sorted and without duplicates", () => {
    const zones = timezoneChoices("Asia/Jakarta", () => ["Europe/Paris", "Asia/Makassar", "Asia/Jakarta", "Asia/Makassar"]);
    expect(zones).toEqual(["Asia/Jakarta", "Asia/Makassar", "Europe/Paris"]);
  });

  test("always keeps the saved zone, even one this browser does not list", () => {
    expect(timezoneChoices("Asia/Pontianak", () => ["Asia/Jakarta"])).toEqual(["Asia/Jakarta", "Asia/Pontianak"]);
  });

  test("falls back to the Indonesian zones when the browser cannot list any", () => {
    expect(timezoneChoices("Asia/Jakarta", () => [])).toEqual(["Asia/Jakarta", "Asia/Jayapura", "Asia/Makassar"]);
    expect(timezoneChoices("Asia/Jakarta", () => {
      throw new Error("not supported");
    })).toEqual(["Asia/Jakarta", "Asia/Jayapura", "Asia/Makassar"]);
  });
});
