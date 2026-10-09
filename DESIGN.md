---
name: Tepati
description: "A study pact kept like a bank passbook, with dated printed lines and a saldo that is always the sum of the lines above it."
colors:
  ground: "oklch(0.966 0.007 145.5)"
  surface: "oklch(0.985 0.004 145)"
  sunken: "oklch(0.94 0.009 150)"
  rule: "oklch(0.867 0.013 172.3)"
  rule-strong: "oklch(0.6 0.02 180)"
  ink: "oklch(0.244 0.025 191.3)"
  muted: "oklch(0.44 0.02 190)"
  cover: "oklch(0.322 0.048 192.9)"
  cover-ink: "oklch(0.955 0.012 150)"
  primary: "oklch(0.322 0.048 192.9)"
  teal-tint: "oklch(0.93 0.02 185)"
  teal-text: "oklch(0.322 0.048 192.9)"
  debit: "oklch(0.5 0.182 29.5)"
  credit: "oklch(0.491 0.09 172.2)"
  stamp: "oklch(0.457 0.215 277)"
  stamp-tint: "oklch(0.94 0.025 277)"
  today: "oklch(0.924 0.115 95.7)"
  member-a: "oklch(0.511 0.086 186.4)"
  member-b: "oklch(0.459 0.17 3.8)"
  ground-dark: "oklch(0.185 0.02 200)"
  surface-dark: "oklch(0.225 0.022 200)"
  sunken-dark: "oklch(0.16 0.02 200)"
  rule-dark: "oklch(0.32 0.02 195)"
  rule-strong-dark: "oklch(0.52 0.03 190)"
  ink-dark: "oklch(0.93 0.022 95)"
  muted-dark: "oklch(0.75 0.02 170)"
  cover-dark: "oklch(0.25 0.04 195)"
  cover-ink-dark: "oklch(0.95 0.012 150)"
  primary-dark: "oklch(0.74 0.085 186)"
  teal-tint-dark: "oklch(0.3 0.04 190)"
  teal-text-dark: "oklch(0.84 0.07 186)"
  debit-dark: "oklch(0.74 0.14 25)"
  credit-dark: "oklch(0.78 0.11 165)"
  stamp-dark: "oklch(0.56 0.2 277)"
  stamp-tint-dark: "oklch(0.3 0.07 277)"
  today-dark: "oklch(0.86 0.11 95.7)"
  member-a-dark: "oklch(0.76 0.1 186)"
  member-b-dark: "oklch(0.74 0.15 5)"
  selection: "oklch(0.85 0.06 188)"
  selection-dark: "oklch(0.46 0.07 188)"
typography:
  display:
    fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.875rem"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.025em"
  headline:
    fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.55
    letterSpacing: "-0.025em"
  body:
    fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 500
    lineHeight: 1.25
  amount:
    fontFamily: "Geist Mono, ui-monospace, Cascadia Mono, monospace"
    fontSize: "2.25rem"
    fontWeight: 600
    lineHeight: 1
    fontFeature: "'tnum'"
  data:
    fontFamily: "Geist Mono, ui-monospace, Cascadia Mono, monospace"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.4
    fontFeature: "'tnum'"
  small:
    fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 500
    lineHeight: 1.45
  micro:
    fontFamily: "Geist Mono, ui-monospace, Cascadia Mono, monospace"
    fontSize: "11px"
    fontWeight: 600
    lineHeight: 1
  prose-heading:
    fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 600
    lineHeight: 1.4
  prose-subheading:
    fontFamily: "Geist, ui-sans-serif, system-ui, sans-serif"
    fontSize: "1.0625rem"
    fontWeight: 600
    lineHeight: 1.45
rounded:
  control: "6px"
  panel: "10px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "16px"
  lg: "24px"
  xl: "32px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.cover-ink}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "44px"
  button-decision:
    backgroundColor: "{colors.stamp}"
    textColor: "{colors.cover-ink}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "44px"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "44px"
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "0 12px"
    height: "44px"
  panel:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.panel}"
    padding: "20px 24px"
  header-band:
    backgroundColor: "{colors.cover}"
    textColor: "{colors.cover-ink}"
    rounded: "{rounded.panel}"
    padding: "24px 28px"
  badge-stamp:
    backgroundColor: "{colors.stamp-tint}"
    textColor: "{colors.stamp}"
    rounded: "{rounded.control}"
    padding: "2px 8px"
  today-mark:
    backgroundColor: "{colors.today}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "4px 10px"
---
# Design System: Tepati

## Overview

**Creative North Star: "Buku Tabungan"**

Tepati is a bank passbook for a promise between two students. Every coin that moves is a dated, printed line with a debit, a credit and the saldo after it, and the saldo is always the sum of the lines above it. The interface is an Operate surface: a person submits proof in under a minute on a phone at night, or clears a short review queue between tasks, and can explain every coin in the pot afterwards. Density is medium-high and nothing is decorative unless it carries meaning.

The cover of the passbook is a deep teal and owns the shell: the rail, the header bands and the primary buttons. The ground is a cool mint security paper, never cream. Ink is a green-black. The dark theme is the same book read under a desk lamp: a blue-green ground, warm lamp-lit ink, and every accent re-picked for contrast instead of inverted. The product refuses the habit-tracker default: no streak flames, no confetti, no green-check heatmaps, no pastel gamified cards.

**Key Characteristics:**
- One teal cover colour for chrome; four meaning colours (debit, credit, stamp violet, highlighter) that are never used for decoration.
- Geist for words, Geist Mono with tabular figures for every amount, date and countdown.
- Radius is 6px on controls and 10px on panels, nothing else.
- Flat paper with hairline rules; depth is a tinted 1px shadow on panels and a soft one on overlays.
- The signature move is the passbook print: a new line slides in under the last one, its digits type left to right, and the saldo rolls to its new value.

## Colors

Every colour is one `light-dark()` pair in `apps/web/src/styles/tokens.css`; the frontmatter carries the light value and a `-dark` partner. A test parses the file and fails if a text pair drops below WCAG AA.

### Primary
- **Passbook Cover Teal** (`oklch(0.322 0.048 192.9)`, light; `oklch(0.74 0.085 186)` as the dark button fill): the rail, the header bands, primary buttons and link text. In the dark theme the cover stays deep (`oklch(0.25 0.04 195)`) and the buttons turn light teal.

### Secondary
- **Stamp Violet** (`oklch(0.457 0.215 277)`): human decisions only: approve, reject, override, dispute, sign the terms, mark paid, confirm receipt, and the "approved" status. It is the ink of the rubber stamp.

### Tertiary
- **Highlighter Yellow** (`oklch(0.924 0.115 95.7)`): today. The date chip on the Today band and today's cell in the calendar, nothing else.
- **Member Teal** (`oklch(0.511 0.086 186.4)`) and **Member Berry** (`oklch(0.459 0.17 3.8)`): one fixed line colour per member, in avatars, underlines of names, calendar marks and ledger attributions, on every screen.

### Neutral
- **Security Paper** (`oklch(0.966 0.007 145.5)`): the page ground. `surface` (`oklch(0.985 0.004 145)`) is panels and fields; `sunken` (`oklch(0.94 0.009 150)`) is hover and wells.
- **Ink** (`oklch(0.244 0.025 191.3)`) and **Muted** (`oklch(0.44 0.02 190)`): text. Dark ink is warm lamp light (`oklch(0.93 0.022 95)`).
- **Rule** (`oklch(0.867 0.013 172.3)`) and **Rule Strong** (`oklch(0.6 0.02 180)`): hairlines between lines, and the border of controls.
- **Debit Red** (`oklch(0.5 0.182 29.5)`) and **Credit Green** (`oklch(0.491 0.09 172.2)`): coins only. Never the only signal: a sign, the D or K label and the word travel with them.

- **Selection Teal** (`oklch(0.85 0.06 188)` light, `oklch(0.46 0.07 188)` dark): the text-selection highlight, themed so it is not the browser blue.

### Named Rules
**The Coins-Only Rule.** Red and green mean coins leaving and entering the pot. A status, a check mark or a success toast is never green or red for its own sake.
**The Stamp Rule.** Violet is reserved for a human decision. If no person decided anything, the element is not violet.
**The Only-Today Rule.** Yellow marks today and nothing else on any screen.
**The Fixed Line Rule.** A member keeps the same line colour in every place they appear.

## Typography

**Display Font:** Geist (self-hosted through the `geist` package, with `ui-sans-serif, system-ui`)
**Body Font:** Geist
**Label/Mono Font:** Geist Mono (with `ui-monospace, Cascadia Mono`), tabular figures

**Character:** a plain, legible sans for words and a printed, fixed-width mono for everything that is a number or a time, so columns line up like ledger lines. `font-variant-numeric: tabular-nums` is on at the root.

### Hierarchy
- **Display** (600, 1.875rem, 1.2, -0.025em): the page title (`h1`) on app screens.
- **Headline** (600, 1.125rem, tracking -0.025em): section titles (`h2`) such as "Buku koin", "Penyelesaian".
- **Body** (400, 15px, 1.5): running text and list rows. Measure stays under about 65 characters in prose blocks.
- **Label** (500, 13px): chips, column heads, hints.
- **Amount** (Mono 600, up to 2.25rem): the saldo and settlement amounts, with the unit "koin" and the rupiah equivalent in smaller mono.
- **Data** (Mono 400, 13px): dates, times, counts, countdown cells.
- **Small** (500, 0.875rem, 1.45): secondary lines under a title, hints, compact list metadata.
- **Micro** (Mono 600, 11px): the D and K marks on an amount and the unread count on the bell; nothing else is this small.
- **Prose heading and subheading** (600, 1.25rem and 1.0625rem): headings inside a proof the doer wrote, in the editor and when it is read (`.proof-prose`).

### Named Rules
**The Printed Number Rule.** Every amount, date and countdown is Geist Mono with tabular figures. Numbers are never set in the sans.
**The Fixed Cell Rule.** Countdown digits sit in fixed-width cells so the clock does not jitter as it ticks.

## Layout

A shell of a cover-teal rail on the left (240px, from 1024px up) and a bottom tab bar (64px) below that. The main column is at most 64rem wide with 16, 32 and 48px side padding by breakpoint. Spacing follows Tailwind's 4px scale; groups are tight and sections are separated by generous space, with more above a heading than below it. Touch targets are 44px (36px for small controls on desktop). Today and the pact page become two columns on desktop (what needs doing on the left, the book on the right) and one on a phone, where the header band with the countdown comes first. Pages are usable from 360px.

## Elevation & Depth

Flat paper. Surfaces rest on the ground with a hairline rule; there are no shadows on cards or rows. Two tinted shadows exist: `shadow-panel` (`0 1px 2px` tinted to the paper) for the rare raised panel, and `shadow-overlay` (a 2px and a 16px soft layer) for dialogs, sheets and menus. In the dark theme a border does the work and shadows nearly vanish.

### Named Rules
**The Flat Paper Rule.** A surface is flat at rest. Depth appears only on something that floats above the page (a dialog, a sheet, a menu).
**The Tinted Shadow Rule.** A shadow is tinted to the paper hue and has an offset and a blur. No pure black, no hard offset.

## Shapes

Two radii: 6px on controls, chips and cells; 10px on panels and header bands. Borders are 1px hairlines in `rule` or `rule-strong`. The guilloche, a fine interlocking radial linework one device pixel thick in a low-contrast tint, appears on the header bands (Today, a pact, the sign-in cover) and nowhere else: not on the rail, not on panels.

## Components

### Buttons
- **Shape:** 6px radius, 44px tall (36px small), 150ms colour transition, a 1px press.
- **Primary:** cover teal fill with light text; the dark theme uses a light teal fill with dark text.
- **Decision:** stamp violet fill (approve, mark paid, confirm); `decision-quiet` is an outlined violet version (reject, dispute).
- **Secondary / Ghost:** surface with a strong rule; ghost has no border and a sunken hover.
- **Disabled / Loading:** half opacity and no pointer; loading keeps the label and swaps the icon for a spinner.

### Chips and status
- **Style:** 13px label, 6px radius, a leading Phosphor icon, a tint of the status colour. A status is always an icon and a word, never a colour alone (Disetujui is violet because a person decided; Terlewat is red because it moved coins).

### Panels
- **Corner style:** 10px. **Background:** surface. **Border:** a strong rule. **Padding:** 20 to 24px. One level only: no card inside a card.

### Inputs / Fields
- **Style:** 44px, strong-rule border, surface fill, 6px radius, a label above and a hint under it.
- **Focus:** a 2px ring in the ring colour. **Error:** the border and ring turn debit red with a message that names the problem and the way out.

### Navigation
- **Rail (desktop):** cover teal, four items with Phosphor icons, the active item on a lighter plate; the bell and the person sit at its ends. **Tab bar (phone):** the same four destinations, 64px, labels always visible.

### Header band
- A cover-teal panel with the guilloche, the page or pact title and, on Today, the date in highlighter yellow and the countdown to the next cutoff in fixed mono cells.

### Passbook (signature component)
- The coin book is a table: date, description, debit, credit and saldo, with a hairline under each line. A new line slides in under the last one in 240ms, its digits type in over 320ms, and the saldo column rolls to its new value; with reduced motion all of it appears at once. The saldo is a server value, never added up in the browser.

## Do's and Don'ts

### Do:
- **Do** show a coin amount with its sign, the D or K label and the word, so colour is never the only signal.
- **Do** set every number, date and countdown in Geist Mono with tabular figures.
- **Do** keep violet for human decisions, yellow for today and the member colours fixed.
- **Do** draw icons from Phosphor in one weight, and give each status an icon and a word.
- **Do** underline text links (a quiet teal underline that strengthens on hover): on touch there is no hover.
- **Do** write copy in Indonesian first, peer to peer, with controls that name their action and errors that name the problem and the recovery.
- **Do** respect reduced motion: durations and delays go to zero and the result is shown at once.

### Don't:
- **Don't** add streak flames, confetti, green-check heatmaps or gamified pastel cards.
- **Don't** use red or green for anything but coins, or violet for anything a person did not decide.
- **Don't** put the guilloche on the rail, panels or rows.
- **Don't** nest cards, put a kicker or eyebrow above a heading, or use gradient text, hard offset shadows or emoji as icons.
- **Don't** compute the balance in the browser: it is the server's sum of the ledger lines.
- **Don't** introduce a third radius or a colour outside the tokens (the build fails on a stray default palette class).
