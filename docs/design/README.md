# Design workflow and direction

## Sources of truth, in priority order

1. **`PRODUCT.md`**: users, principles, voice, and accessibility. Written with impeccable `init`.
2. **`.impeccable/surfaces/apps-web-src-app-app.md`**: the **direction contract** for the signed-in app shell. It has six blocks: THESIS, OWN-WORLD, STORY, FIRST VIEWPORT, SIGNATURE MOVE/FORM, and FINISH. Every UI task must follow it.
3. **`DESIGN.md`** and `.impeccable/design.json` don't exist yet. The impeccable documenter writes them **from the built UI** at the end of PLAN 8.2. After that, DESIGN.md is the token authority. Until then, the contract's hex values are.
4. `docs/adr/0009-visual-direction.md`: why this direction was chosen and what was rejected.

## The direction in one paragraph

**Buku Tabungan.** The coin pot is a bank passbook. Every movement is a dated line with debit/credit/saldo in tabular mono figures, and when a new line lands it types itself in (the *passbook print* signature move). The shell is passbook-cover teal on a cool mint security-paper ground, not cream. Stamp-ink violet appears only on human decisions (approve, reject, override, signing terms). Highlighter yellow marks today and nothing else. Each member keeps one fixed line colour everywhere. The result should feel serious, honest about money, and calm under a deadline. It must not feel like a gamified habit tracker: no flames, no confetti, no badges.

## Process for agents

- **New screen inside the app shell:** follow impeccable's *extend an existing surface* path. Inherit the world and decide only the content, hierarchy, and states. There is no new direction round.
- **A new surface with a different mode** (landing page = Persuade, help docs = Read): run `/impeccable shape <surface>`. It gets its own surface brief and direction round.
- **Build path:** code-first (`.impeccable/config.json`). Don't generate comp images unless the user asks.
- **Quality loop:** build → screenshots (390 + 1440, light + dark) → `impeccable detect --json` → at most two fix rounds → finish reviewer → documenter. Use *taste-skill* §9 as the banned-patterns checklist and *web-design-guidelines* for the final audit.

## References (inspiration, not authority)

- `references/wise.DESIGN.md` comes from [VoltAgent/awesome-design-md](https://github.com/VoltAgent/awesome-design-md) at commit `13be5c0`. It is an analysis of Wise's money UI. Use it **only** for craft in money presentation: amount hierarchy, positive and negative semantics, and transfer-status clarity. Don't import its palette, type, or lime accent, because the passbook world owns the look.
- Research notes on comparable products (stickK referees, Beeminder, proof-based habit apps like ACTRA and SnapHabit, and TinyAct's shared streaks) are summarised in the founding conversation. The useful lessons are already in PRODUCT.md and SPEC.

## Open design items

- Logo and wordmark for "Tepati". Use a text wordmark in Geist semibold until they exist.
- Landing page (PLAN 9.2).
- Email templates should be styled to match the passbook world (simple, mono amounts).
