import { describe, expect, test } from "bun:test";
import { readyIds, trayCounts, trayReducer, type TrayAction, type TrayItem } from "./tray";

const add = (localId: string, kind: TrayItem["kind"] = "image"): TrayAction => ({
  type: "add",
  item: { localId, name: `${localId}.png`, kind, bytes: 1000 },
});

describe("trayReducer", () => {
  test("an added file starts working and can be told apart by its local id", () => {
    expect(trayReducer([], add("a"))).toEqual([
      { localId: "a", name: "a.png", kind: "image", bytes: 1000, state: "working", step: "preparing", progress: 0 },
    ]);
  });

  test("upload progress is clamped between 0 and 1", () => {
    let s = trayReducer([], add("a"));
    s = trayReducer(s, { type: "progress", localId: "a", progress: 0.4 });
    expect(s[0]).toMatchObject({ state: "working", step: "uploading", progress: 0.4 });
    s = trayReducer(s, { type: "progress", localId: "a", progress: 7 });
    expect(s[0]).toMatchObject({ progress: 1 });
  });

  test("after the PUT the file waits for the server to process it", () => {
    let s = trayReducer([], add("a"));
    s = trayReducer(s, { type: "uploaded", localId: "a", attachmentId: "att-1" });
    expect(s[0]).toMatchObject({ state: "working", step: "processing", attachmentId: "att-1" });
  });

  test("ready keeps the attachment id and the thumbnail", () => {
    let s = trayReducer([], add("a"));
    s = trayReducer(s, { type: "uploaded", localId: "a", attachmentId: "att-1" });
    s = trayReducer(s, { type: "ready", localId: "a", thumb: "https://x/t.webp" });
    expect(s[0]).toMatchObject({ state: "ready", attachmentId: "att-1", thumb: "https://x/t.webp" });
  });

  test("rejected carries the server's reason", () => {
    let s = trayReducer([], add("a"));
    s = trayReducer(s, { type: "rejected", localId: "a", reason: "not a real image" });
    expect(s[0]).toMatchObject({ state: "rejected", reason: "not a real image" });
  });

  test("failed carries a message key and can be retried back to working", () => {
    let s = trayReducer([], add("a"));
    s = trayReducer(s, { type: "failed", localId: "a", messageKey: "Errors.network" });
    expect(s[0]).toMatchObject({ state: "failed", messageKey: "Errors.network" });
    s = trayReducer(s, { type: "retry", localId: "a" });
    expect(s[0]).toMatchObject({ state: "working", step: "preparing", progress: 0 });
  });

  test("remove drops only that file", () => {
    let s = trayReducer([], add("a"));
    s = trayReducer(s, add("b"));
    expect(trayReducer(s, { type: "remove", localId: "a" }).map((i) => i.localId)).toEqual(["b"]);
  });

  test("an event for an unknown file changes nothing", () => {
    const s = trayReducer([], add("a"));
    expect(trayReducer(s, { type: "ready", localId: "zzz", thumb: null })).toEqual(s);
  });

  test("an existing attachment (editing a proof) enters as ready", () => {
    const s = trayReducer([], { type: "existing", item: { localId: "e", attachmentId: "att-9", name: "foto", kind: "image", bytes: 5, thumb: null } });
    expect(s[0]).toMatchObject({ state: "ready", attachmentId: "att-9" });
  });
});

describe("selectors", () => {
  const mk = (): TrayItem[] => {
    let s: TrayItem[] = [];
    for (const id of ["w", "r", "x", "f", "p"]) s = trayReducer(s, add(id));
    s = trayReducer(s, { type: "uploaded", localId: "r", attachmentId: "att-r" });
    s = trayReducer(s, { type: "ready", localId: "r", thumb: null });
    s = trayReducer(s, { type: "rejected", localId: "x", reason: "bad" });
    s = trayReducer(s, { type: "failed", localId: "f", messageKey: "Errors.network" });
    s = trayReducer(s, { type: "uploaded", localId: "p", attachmentId: "att-p" });
    return s;
  };
  test("counts ready, still-working and the ones that will not be sent", () => {
    expect(trayCounts(mk())).toEqual({ ready: 1, pending: 2, rejected: 1, failed: 1 });
  });
  test("only ready attachments are sent with the proof", () => {
    expect(readyIds(mk())).toEqual(["att-r"]);
  });
});
