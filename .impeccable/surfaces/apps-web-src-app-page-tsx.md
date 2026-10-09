---
version: 1
slug: "apps-web-src-app-page-tsx"
primary_target: "apps/web/src/app/page.tsx"
related_targets: ["apps/web/src/components/landing"]
---

# Surface brief: landing page (`/`, signed out)

Scope: the page a visitor without a session sees at `/`. A signed-in visitor still goes to `/today`. Visitor mode: **Persuade**. It extends the established world (Buku Tabungan, see `DESIGN.md` and `.impeccable/surfaces/apps-web-src-app-app.md`); it does not replace or amend it.

Audience and job: students who have heard of Tepati from a friend or found it cold, usually on a phone. They must understand in one viewport what it is (a pact between two peers with a daily proof and a coin pot), why it is not a habit tracker (stakes move both ways, every coin is a printed line), and what it is not (no real money passes through the app). Action: **Buat akun**, then create a contract. Second action: **Masuk**. Confirmed with the owner (2026-10-09): the primary action is registering; the money honesty line is bright and near the top; Indonesian first, English alongside; no user numbers, testimonials or logos (none exist); example data is synthetic and labelled "contoh".

Proof and content: a sample month of a 28-day pact drawn as the product draws it (a stamped calendar and the printed lines under it), the four steps of a pact as dated lines, the rules that protect both people (auto-approval, disputes, a limited and logged override, rest days, both sign the terms, nobody reviews their own proof). Nothing is claimed that the product does not do.

Constraints: WCAG 2.2 AA, 390 to 1440, reduced motion, light and dark. No new colours, radii or faces; the tokens of `DESIGN.md` only. Server component, no data fetch (static copy and a static sample). Copy in `messages/id.json` and `en.json`.

## Direction contract

THESIS: A pact is a month of days, each one stamped, and the pot is the running total of what those stamps cost. The page shows that one month, stamped, with its saldo printed under it, before it explains anything. It refuses the SaaS landing default: a headline over subcopy and a button, then a row of three equal cards with icons, then a testimonial strip.

OWN-WORLD: The passbook cover in teal (`cover`) with the guilloche carries the top third and the final call to action; the rest is mint security paper (`ground`, `surface`, hairline rules). Stamp violet marks only the human decisions in the sample (approved, the review), debit red and credit green appear only as coins on the printed lines, and highlighter yellow marks one day, "today", in the sample month and nowhere else. Member teal and berry are the two people's fixed colours in the sample. Geist for words, Geist Mono with tabular figures for every date, count and amount. Radius 6 and 10. Depth is the paper's hairline, not shadow.

STORY: In one viewport the visitor reads the hook, sees a stamped month and understands from the stamps what a missed day costs, reads in one plain line that coins are only a record between friends, and sees the button. Further down they read how a pact runs as dated lines, which rules keep both people honest, and meet the same button again.

FIRST VIEWPORT: Mobile 390: the teal cover band (wordmark left, Masuk right), the hook in display size, one supporting sentence, the money line in plain ink on a light chip, and the full-width "Buat akun" (then a quieter "Masuk"). The top of the sample month, headed "Contoh: kontrak 28 hari", peeks under the button so the stamps invite a scroll. Desktop 1440: the cover band holds the hook, the sentence, the money line and the button on the left half; the sample month, as a paper page set over the band's lower edge, on the right half, with its printed lines under the grid, so the first screen already shows days stamped and a saldo.

FORM: The passbook's own page, calendar-first (candidate 4 of 7 on the ordered list: 1 cover opening to a spread; 2 a ledger that prints itself; 3 the agreement sheet with two signatures; 4 a stamped calendar over its running saldo; 5 a deposit slip; 6 the end-of-pact statement; 7 one day on a shared timeline). Seed key e766db7d, dealt 4, 2, 6; the owner locked the stamped calendar. SIGNATURE MOVE: the sample month stamps itself day by day and its saldo lines print in under the grid (the product's passbook print), once, when it comes into view; with reduced motion it is all there at once.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Unresolved

- No logo (a text wordmark is used, as in the app).
- English copy is a straight translation; the owner has not reviewed it.
- No SEO and social-card work (title and description only); no analytics.
