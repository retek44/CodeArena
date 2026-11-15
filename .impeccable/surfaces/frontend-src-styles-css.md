---
version: 1
slug: "frontend-src-styles-css"
primary_target: "frontend/src/styles.css"
related_targets: ["frontend/src/App.tsx","frontend/src/components/Layout.tsx"]
---

# CodeArena SPA

Scope: the whole React SPA (problems, problem + editor, submissions, contests and
leaderboard, admin, sign-in). Visitor mode: Operate.

Audience and job: practising programmers and contestants in a solving loop (read, code,
submit, read per-test-case results, fix); admins preparing problems. Main scene: a laptop
at home in the evening, long sessions, dim room.

Constraints: every behaviour, text, URL, accessible role and name used by the tests stays;
no gamification, no generic dashboard template, no hacker or terminal costume; WCAG 2.1
AA; fonts self-hosted (CSP `font-src 'self'`).

## Direction contract

THESIS: A submission departs and its test cases arrive like trains on a station board:
each verdict flips into a split-flap cell. Refuses the category default, a dark IDE with
green and red pills, and the opposite, a white SaaS dashboard.

OWN-WORLD: Board black ground from a fixed seven-step ramp (#0e1013 #15171a #1c1f23 #262a2f #3a3f46 #9ba1aa #ecebe6), flap
white text, Barlow Condensed in flap cells and labels, Barlow for prose, JetBrains Mono for
code. Colour lives only inside state cells: signal amber (queued, running, primary
action), platform green (accepted, solved), signal red (rejected). One 8px cell module
sizes flaps, rows and gaps; numerals are always tabular.

STORY: The visitor sees the problem and the editor, submits, and watches each test case
flip from QUEUED to its verdict with time and memory; a rejection shows which case and why.
Contests read as a departures board ranked by solved and penalty.

FIRST VIEWPORT: Problem page. Board-black top bar: CodeArena in flap cells, nav (Problems,
Contests, My submissions, Admin), user and Sign out right. Left column 520px: limits as
flap chips, title, statement, samples. Right: language picker and amber Submit, editor,
full height and sticky beside the statement. Submit opens the submission (URL and flow
unchanged): a result cell that flips from IN QUEUE to JUDGING to the verdict, then the
results board, one row per case (case, verdict flap, ms, KB) flipping in as cases arrive.

FORM: Departures Board (split-flap station board), position 4 of 7 on the ordered list;
seed:0493f4a13c35417a. Signature move: status cells flip (split-flap animation) when a
verdict or phase changes; reduced motion swaps instantly.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Raises (from the declined challengers)

- Lexicon: colour only inside state cells.
- Character goods catalog: fixed furniture per row.
- Bitmap specimen: one board face at whole-step sizes, tabular numerals.
- Crouwel grid: one cell module for flaps, rows and gaps.
- Darkroom record: neutrals only from the fixed ramp.

## Open decisions

- Monaco editor theme: dark, tuned to the board ramp.
