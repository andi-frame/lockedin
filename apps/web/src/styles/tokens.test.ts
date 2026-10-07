import { describe, expect, test } from "bun:test";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { contrast, inGamut, type Oklch } from "../test/oklch";

type Pair = { light: Oklch; dark: Oklch };

const css = readFileSync(join(import.meta.dir, "tokens.css"), "utf8");
const num = String.raw`(-?[\d.]+)`;
const color = String.raw`oklch\(${num}\s+${num}\s+${num}\)`;
const line = new RegExp(String.raw`--color-([\w-]+):\s*light-dark\(\s*${color}\s*,\s*${color}\s*\)`, "g");

const tokens = new Map<string, Pair>();
for (const m of css.matchAll(line)) {
  const [, name, l1, c1, h1, l2, c2, h2] = m;
  tokens.set(name!, {
    light: { l: +l1!, c: +c1!, h: +h1! },
    dark: { l: +l2!, c: +c2!, h: +h2! },
  });
}

const get = (name: string, side: "light" | "dark"): Oklch => {
  const t = tokens.get(name);
  if (!t) throw new Error(`token --color-${name} is missing from tokens.css`);
  return t[side];
};

// [foreground, background, minimum ratio]. 4.5 is WCAG AA for text; 3 is AA for UI shapes and large text.
const pairs: [string, string, number][] = [
  ["ink", "ground", 7],
  ["ink", "surface", 7],
  ["ink", "sunken", 7],
  ["muted", "ground", 4.5],
  ["muted", "surface", 4.5],
  ["placeholder", "surface", 4.5],
  ["placeholder", "sunken", 4.5],
  ["rule-strong", "ground", 3],
  ["rule-strong", "surface", 3],
  ["cover-ink", "cover", 7],
  ["cover-muted", "cover", 4.5],
  ["primary-foreground", "primary", 4.5],
  ["primary-foreground", "primary-hover", 4.5],
  ["debit", "ground", 4.5],
  ["debit", "surface", 4.5],
  ["debit", "debit-tint", 4.5],
  ["credit", "ground", 4.5],
  ["credit", "surface", 4.5],
  ["credit", "credit-tint", 4.5],
  ["stamp-foreground", "stamp", 4.5],
  ["stamp-foreground", "stamp-hover", 4.5],
  ["stamp", "ground", 3],
  ["stamp-text", "stamp-tint", 4.5],
  ["stamp-text", "surface", 4.5],
  ["today-ink", "today", 7],
  ["member-a", "ground", 4.5],
  ["member-a", "surface", 4.5],
  ["member-b", "ground", 4.5],
  ["member-b", "surface", 4.5],
  ["member-a-foreground", "member-a", 4.5],
  ["member-b-foreground", "member-b", 4.5],
  ["ring", "ground", 3],
  ["ring", "surface", 3],
  ["ink", "teal-tint", 7],
  ["teal-text", "teal-tint", 4.5],
];

describe("design tokens", () => {
  test("the file was parsed", () => expect(tokens.size).toBeGreaterThan(25));

  for (const side of ["light", "dark"] as const) {
    describe(`${side} theme`, () => {
      test("every colour is inside sRGB", () => {
        const outside = [...tokens].filter(([, p]) => !inGamut(p[side])).map(([n]) => n);
        expect(outside).toEqual([]);
      });

      for (const [fg, bg, min] of pairs) {
        test(`${fg} on ${bg} is at least ${min}:1`, () => {
          expect(contrast(get(fg, side), get(bg, side))).toBeGreaterThanOrEqual(min);
        });
      }
    });
  }

  test("the dark theme is designed, not inverted: the ground is not the light ink and vice versa", () => {
    const lightGround = get("ground", "light");
    const darkGround = get("ground", "dark");
    const lightInk = get("ink", "light");
    // An inversion would put the light theme's ink hue back as the ground.
    expect(Math.abs(darkGround.h - lightInk.h)).toBeGreaterThan(2);
    expect(darkGround.l).toBeLessThan(0.25);
    expect(lightGround.l).toBeGreaterThan(0.9);
    // Lamp-lit ink is warm (hue near 95), paper ink is cool.
    expect(get("ink", "dark").h).toBeGreaterThan(80);
    expect(get("ink", "dark").h).toBeLessThan(110);
  });

  test("the contract's hex values are kept in the light theme (cover teal, stamp violet, highlighter)", () => {
    expect(get("cover", "light")).toEqual({ l: 0.322, c: 0.048, h: 192.9 });
    expect(get("stamp", "light")).toEqual({ l: 0.457, c: 0.215, h: 277 });
    expect(get("today", "light")).toEqual({ l: 0.924, c: 0.115, h: 95.7 });
  });
});
