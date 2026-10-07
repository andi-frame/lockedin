---
version: 1
slug: "apps-web-src-app-app"
primary_target: "apps/web/src/app/(app)"
related_targets: []
---

# Surface brief: authenticated app shell (Today, Pact, Review)

Scope: the signed-in product (`apps/web/src/app/(app)/**`), covering Today, Pact detail (passbook), Review queue, Check-in detail, and the New pact wizard. Visitor mode: **Operate**. The marketing landing page is a separate Persuade surface and gets its own brief later.

Audience and job: peer students. The doer submits proof before the cutoff, usually on a phone at night. The backer clears a short review queue between tasks. Success: proof is submitted in under a minute; a review decision takes one glance plus one tap; both people can explain every coin in the pot.

Constraints: Bahasa Indonesia first (next-intl). Mobile-first for the doer flows. WCAG 2.2 AA. Amounts never rely on colour alone: always show a sign, D/K labels, and the word. Dark theme is required; it is the passbook read under a desk lamp.

## Direction contract

THESIS: The pot is a passbook (buku tabungan). Every coin movement is a dated, printed line with debit, credit, and balance (saldo), and the balance is always the sum of the lines above it. This surface refuses the habit-tracker default: streak flames, confetti, green-check heatmaps, and gamified pastel cards.

OWN-WORLD: The passbook-cover teal (#0E3B3A) owns the shell: rail, header band, and primary buttons. The ground is a cool mint security-paper tint (#F1F5F1, never cream), with a 1-device-pixel guilloche line pattern in the pot header band only. Ink is #112423, rules are #CBD6D2, debit is #B42318, and credit is #12715B. Stamp-ink violet (#4338CA) is reserved exclusively for human decisions: approve, reject, override, sign terms. A highlighter yellow (#FDE68A) marks "today" and nothing else. Each member keeps one fixed line colour everywhere: teal #0F766E and berry #9D174D. Type is Geist for UI and Geist Mono with tabular figures for every amount, date, and countdown. Radius is 6px on controls and 10px on panels, nothing else. Density is medium-high.

STORY: Within the first glance the visitor knows how long they have until cutoff, whether today is done, what needs their review, and what the pot holds. They act (submit or review) and then see the consequence printed as a line.

FIRST VIEWPORT: On mobile (390), a cover-teal header band shows the pact name and the countdown in fixed-cell mono ("03:41:12 lagi"). Below it sits the "Hari ini" check-in panel: commitment text, status chip, and the primary action "Kirim bukti" full width. Next is "Perlu ditinjau (2)" as compact rows. Last is a mini passbook: the last 3 lines plus the saldo in large mono. On desktop (1440), a teal left rail holds Hari ini, Kontrak, Tinjau, and Pengaturan. The main area has two columns: Today panel plus review rows on the left, the full passbook on the right.

SIGNATURE MOVE: Passbook print. When a settlement, decision, or adjustment lands, its line slides in under the last line and its mono digits type left to right in 320ms, then the saldo column rolls to the new value. Under reduced motion it appears instantly. Raises: countdown digits sit in fixed cells (from split-flap); one fixed line colour per member across calendar, ledger, and avatars (from transit map); stamp violet is reserved for human sign-off (from the design-annual seal); only today wears the highlighter (from orienteering's active-leg purple).

FORM: Buku Tabungan (bank passbook), candidate 3 of 7 on the grounded list (LJK, materai agreement, buku tabungan, KRL board, Rupiah guilloche, rapor, celengan). Seed key 430d207e.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Unresolved

- Logo and wordmark for "Tepati" have not been designed yet; use a text wordmark until then.
- The landing page (Persuade) is out of scope here.
