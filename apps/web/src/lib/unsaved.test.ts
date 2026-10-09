import { expect, test } from "bun:test";
import { beforeUnloadHandler } from "./unsaved";

// A hard exit (closing the tab, reloading, typing another address) loses a half-written proof or a
// half-filled pact; the browser's own prompt is the only guard it allows.
const event = () => {
  const e = { prevented: false, returnValue: undefined as unknown, preventDefault() { this.prevented = true; } };
  return e;
};

test("nothing is asked while there is nothing to lose", () => {
  const e = event();
  beforeUnloadHandler(() => false)(e);
  expect(e.prevented).toBe(false);
  expect(e.returnValue).toBeUndefined();
});

test("the browser's confirmation is asked for while there are unsaved changes", () => {
  const e = event();
  beforeUnloadHandler(() => true)(e);
  expect(e.prevented).toBe(true);
  expect(e.returnValue).toBe("");
});

test("it reads the state when the page is left, not when the handler was made", () => {
  let dirty = false;
  const handler = beforeUnloadHandler(() => dirty);
  const first = event();
  handler(first);
  dirty = true;
  const second = event();
  handler(second);
  expect([first.prevented, second.prevented]).toEqual([false, true]);
});
