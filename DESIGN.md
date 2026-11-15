---
name: CodeArena
description: An online judge set as a night-time station departures board, with split-flap cells for every state.
colors:
  board-black: "#0e1013"
  board-ground: "#15171a"
  row-slate: "#1c1f23"
  flap-slate: "#262a2f"
  rule-grey: "#3a3f46"
  label-grey: "#9ba1aa"
  flap-white: "#ecebe6"
  signal-amber: "#f2b134"
  amber-ink: "#1a1405"
  amber-cell: "#2e2410"
  platform-green: "#52c27f"
  green-cell: "#15291d"
  signal-red: "#f0716a"
  red-cell: "#301a19"
typography:
  display:
    fontFamily: "Barlow Condensed, Arial Narrow, sans-serif"
    fontSize: "40px"
    fontWeight: 600
    lineHeight: 1.15
    letterSpacing: "0.01em"
  headline:
    fontFamily: "Barlow Condensed, Arial Narrow, sans-serif"
    fontSize: "20px"
    fontWeight: 600
    lineHeight: 1.15
    letterSpacing: "0.1em"
  title:
    fontFamily: "Barlow Condensed, Arial Narrow, sans-serif"
    fontSize: "20px"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "0.02em"
  body:
    fontFamily: "Barlow, system-ui, sans-serif"
    fontSize: "16px"
    fontWeight: 400
    lineHeight: 1.6
    fontFeature: "tnum"
  label:
    fontFamily: "Barlow Condensed, Arial Narrow, sans-serif"
    fontSize: "15px"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "0.08em"
  column-head:
    fontFamily: "Barlow Condensed, Arial Narrow, sans-serif"
    fontSize: "13px"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "0.14em"
  control:
    fontFamily: "Barlow, system-ui, sans-serif"
    fontSize: "15px"
    fontWeight: 600
    lineHeight: 1
  code:
    fontFamily: "JetBrains Mono, ui-monospace, Consolas, monospace"
    fontSize: "0.875em"
    fontWeight: 400
    lineHeight: 1.55
rounded:
  cell: "2px"
spacing:
  u1: "8px"
  u2: "16px"
  u3: "24px"
  u4: "32px"
  u5: "40px"
  u6: "48px"
  header: "56px"
components:
  button-default:
    backgroundColor: "{colors.row-slate}"
    textColor: "{colors.flap-white}"
    typography: "{typography.control}"
    rounded: "{rounded.cell}"
    padding: "0 16px"
    height: "40px"
  button-default-hover:
    backgroundColor: "{colors.flap-slate}"
    textColor: "{colors.flap-white}"
  button-primary:
    backgroundColor: "{colors.signal-amber}"
    textColor: "{colors.amber-ink}"
    typography: "{typography.control}"
    rounded: "{rounded.cell}"
    padding: "0 24px"
    height: "40px"
  button-primary-hover:
    backgroundColor: "#f6c25a"
    textColor: "{colors.amber-ink}"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.label-grey}"
    typography: "{typography.control}"
    rounded: "{rounded.cell}"
    padding: "0 16px"
    height: "40px"
  button-quiet-hover:
    backgroundColor: "{colors.row-slate}"
    textColor: "{colors.label-grey}"
  input:
    backgroundColor: "{colors.board-black}"
    textColor: "{colors.flap-white}"
    typography: "{typography.body}"
    rounded: "{rounded.cell}"
    padding: "0 12px"
    height: "40px"
  flap-plain:
    backgroundColor: "{colors.flap-slate}"
    textColor: "{colors.flap-white}"
    typography: "{typography.label}"
    rounded: "{rounded.cell}"
    padding: "0 10px"
    height: "32px"
  flap-ok:
    backgroundColor: "{colors.green-cell}"
    textColor: "{colors.platform-green}"
  flap-bad:
    backgroundColor: "{colors.red-cell}"
    textColor: "{colors.signal-red}"
  flap-run:
    backgroundColor: "{colors.amber-cell}"
    textColor: "{colors.signal-amber}"
  flap-wait:
    backgroundColor: "{colors.row-slate}"
    textColor: "{colors.label-grey}"
  board-row:
    backgroundColor: "{colors.row-slate}"
    textColor: "{colors.flap-white}"
    typography: "{typography.title}"
    rounded: "{rounded.cell}"
    padding: "0 16px"
    height: "48px"
  board-row-hover:
    backgroundColor: "{colors.flap-slate}"
  nav-link:
    textColor: "{colors.label-grey}"
    typography: "{typography.label}"
  nav-link-active:
    textColor: "{colors.flap-white}"
  header:
    backgroundColor: "{colors.board-black}"
    height: "56px"
    padding: "0 32px"
---

# Design System: CodeArena

## Overview

**Creative North Star: "The Departures Board"**

CodeArena is a station board at night. A submission departs, and its test cases arrive like trains: each verdict flips into a split-flap cell with its time and memory beside it. The ground is board black, the text is flap white, and every list is a board of fixed-height rows separated by thin dark gaps. The system is dense, calm and legible for long evening sessions in a dim room.

Colour is rationed. Neutrals come from one seven-step ramp and nothing else. Green and red appear only inside state cells: a verdict, a solved or rejected leaderboard cell, an error strip. Amber does double duty as the "in progress" signal and as the single action and focus colour. Everything is sized from one 8px module, and every numeral is tabular so columns of milliseconds and kilobytes line up.

The world refuses two defaults: the dark IDE with green and red pills, and the white SaaS dashboard. It also refuses a hacker or terminal costume; monospace type appears only where code is shown.

**Key Characteristics:**
- Board-black ground from a fixed seven-step neutral ramp.
- Split-flap cells (two-tone face with a 1px hinge line) carry every state.
- Green and red live only inside state cells; amber marks progress, the primary action and focus.
- One 8px module sizes cells, rows, gaps and padding.
- Barlow Condensed for the board, Barlow for prose, JetBrains Mono for code.
- Flat: no shadows, no blur; depth comes from ramp steps and 2px row gaps.
- One motion: the flap flip, only when state changes while the page is open.

## Colors

A dark, near-neutral board with three signal colours, each paired with its own dim cell ground.

### Primary
- **Signal Amber** (`--amber`): the primary action (Submit, Save), the in-progress state (JUDGING, pending leaderboard cell), and the whole interaction layer: focus ring, active nav bar, link hover, text selection, caret, checkbox accent.
- **Amber Ink** (`--amber-ink`): text on amber, used on the primary button, selection and skip link.
- **Amber Cell** (`--amber-cell`): the dim ground behind amber text inside a running or pending cell.

### Secondary
- **Platform Green** (`--green`) on **Green Cell** (`--green-cell`): Accepted verdicts, solved leaderboard cells, the "registered" confirmation.

### Tertiary
- **Signal Red** (`--red`) on **Red Cell** (`--red-cell`): rejected verdicts, rejected leaderboard cells, form and request errors. The editor error strip uses Red Cell as its ground.

### Neutral
The board ramp, darkest to lightest (`--r0` to `--r6`):
- **Board Black** (`--r0`): header, editor pane, code blocks, inputs, the result board, the flap hinge line.
- **Board Ground** (`--r1`): page background, work bar, option lists, scrollbar track.
- **Row Slate** (`--r2`): board rows, default buttons, panels (auth, test case, statement editor), the waiting flap.
- **Flap Slate** (`--r3`): the plain flap face, hovered rows, inline code, 1px dividers.
- **Rule Grey** (`--r4`): control borders, link underlines at rest, scrollbar thumb.
- **Label Grey** (`--r5`): labels, section headings, column heads, metadata, rank numbers, quiet buttons.
- **Flap White** (`--r6`): all primary text.

### Named Rules
**The Seven-Step Ramp Rule.** Every neutral comes from `--r0` to `--r6`. A new grey is not added; pick the nearest step.

**The Signal-In-Cells Rule.** Green and red appear only inside state cells and error messages, always on their own cell ground. Amber is the one exception that leaves the cells, and only for the primary action and interaction feedback.

**The Code-And-Label Rule.** Colour is never the only signal. A verdict cell always carries its code (WA) and its label (Wrong answer); where space forces the label out of sight, it stays for screen readers.

## Typography

**Display Font:** Barlow Condensed (with Arial Narrow, sans-serif)
**Body Font:** Barlow (with system-ui, sans-serif)
**Label/Mono Font:** JetBrains Mono (with ui-monospace, Consolas, monospace), for code only

**Character:** Barlow Condensed is the board face: tall, narrow, uppercase and tracked in cells and labels, like painted station lettering. Barlow is its humanist sibling for statements and controls, so the pair reads as one family. All three fonts are self-hosted through @fontsource because the CSP allows fonts only from the same origin.

### Hierarchy
- **Display** (600, 40px, 1.15): page titles. Drops to 28px below 960px.
- **Headline** (600, 20px, 0.1em tracking, uppercase, Label Grey): section headings such as Examples, Leaderboard, Compiler output.
- **Title** (600, 20px, 1.2): the name or number in a board row, and the figures in the results board.
- **Body** (400, 16px, 1.6): statements and prose. Statements run at 1.65 line height and a 68ch measure.
- **Label** (600, 15px, 0.08em to 0.12em tracking, uppercase): flap cells, sub-headings, captions, legends, nav links, file buttons.
- **Column head** (600, 13px, 0.14em tracking, uppercase, Label Grey): table headers.
- **Control** (Barlow 600, 15px, 1): button text.
- **Code** (JetBrains Mono 400, 0.875em, 1.55): code blocks, inline code, sample input and output, the checker textarea.

### Named Rules
**The Tabular Rule.** Numerals are tabular everywhere (set on the root), so times, memory, ranks and penalties align in columns.

**The Board Face Rule.** Anything that names a state, a column or a section is Barlow Condensed in uppercase with positive tracking; anything a person reads as a sentence is Barlow in sentence case.

## Layout

Everything snaps to the 8px module (`--u`): cells are 4u (32px) tall, controls 5u (40px), board rows 6u (48px), and gaps and padding are multiples or half-multiples of the module. The header is a fixed 56px sticky bar.

Content pages sit in a 1200px column with 40px top and 32px side padding. The problem page breaks out to full width: the statement column takes 360px to 560px on the left with a 1px divider, and the editor pane fills the rest, sticky beside the statement at full viewport height minus the header.

Boards are tables with separated rows: a 2px vertical gap between rows, rows 48px tall, content vertically centred, and no wrapping. Long names end in an ellipsis instead of breaking. The results board also separates columns by 2px, so every cell reads as its own flap.

Below 960px the header wraps with the nav on its own scrolling line, the problem page stacks with a 420px editor, page padding drops to 16px at the sides, two-column samples and test cases become one column, and tables scroll sideways. The results board stays a table and hides verdict labels visually. The leaderboard keeps rank and user sticky on the left while the problem cells scroll beneath.

### Named Rules
**The One Module Rule.** Sizes are expressed as multiples of `--u`. A row is 6u, a control 5u, a cell 4u.

**The No-Wrap Row Rule.** A board row is one line. Truncate with an ellipsis; never wrap or hyphenate a row.

## Elevation & Depth

The board is flat. There are no drop shadows, no blur and no translucent layers. Depth comes from ramp steps (a Row Slate row on Board Ground, a Board Black pane beside it), 1px Flap Slate dividers, and the 2px gaps that separate board rows. The only surface gradient is the flap face itself (the select chevron is drawn with gradients, but as an icon, not a surface).

### Shadow Vocabulary
- **Active nav bar** (`box-shadow: inset 0 -3px 0 var(--amber)`): a 3px amber bar under the current section. An indicator, not elevation.
- **Sticky column edge** (`box-shadow: 2px 0 0 var(--r1)`): on a phone, separates the sticky user column from the scrolled cells.

### Named Rules
**The Flat Board Rule.** Surfaces never float. If something needs separation, use a ramp step or a 2px gap.

## Shapes

Corners are barely softened: 2px on every cell, row, button, input, panel and code chip, matching a painted metal flap. Row radii are applied to the first and last cell so a row reads as one strip. Borders are 1px, in Rule Grey on controls and Flap Slate on dividers and panes.

The flap face is the signature shape: a vertical split with the upper half 5% lighter than the lower half and a 1px Board Black hinge line at exactly 50%, behind the glyphs. The wordmark is the same shape at small scale: one 20 by 28px flap per letter, 2px apart.

## Components

### Buttons
Square, compact and quiet; amber is reserved for the one action that commits.
- **Shape:** nearly square corners (2px), 40px tall.
- **Default:** Row Slate ground, Rule Grey 1px border, Flap White text, 16px side padding.
- **Primary:** Signal Amber ground and border, Amber Ink text, 24px side padding, slight tracking (0.02em). One per view: Submit, Save, Sign in.
- **Quiet:** transparent ground and border, Label Grey text; used for Sign out and secondary actions inside a row.
- **Hover / Focus:** hover steps the ground up one ramp step and the border to Label Grey; primary lightens to a paler amber. Transitions are 160ms ease-out on colour only. Focus is a 2px amber outline at 2px offset. Disabled is 50% opacity.

### Inputs / Fields
- **Style:** Board Black ground, Rule Grey 1px border, 2px corners, 40px tall, 12px side padding. Textareas match, with code textareas in JetBrains Mono 14px.
- **Select:** the native arrow is replaced by a small chevron drawn from Label Grey, turning Flap White on hover.
- **File input:** the picker button is a Flap Slate label in the board face.
- **Hover / Focus:** border to Label Grey on hover; amber focus outline.
- **Disabled:** Row Slate ground, Label Grey text.
- **Labels:** Barlow 15px in Label Grey, above the field.

### Navigation
- **Header:** sticky Board Black bar, 56px, with a 1px Flap Slate rule below. Wordmark left, nav, then user name and Sign out on the right.
- **Wordmark:** CodeArena set one letter per flap (Barlow Condensed 700, 18px); the letters turn amber on hover.
- **Links:** board face 15px uppercase, 0.1em tracking, Label Grey; Flap White on hover; the active section is Flap White with the 3px amber bar.
- **Mobile:** the nav moves to its own line and scrolls sideways.

### Split-Flap Cell (signature)
The board's one signature component, used for every state: verdicts, submission phases, limits, difficulty, time and memory.
- **Shape:** 32px tall (40px in the result board, 48px in the results board), 10px side padding, 2px corners, the hinged two-tone face.
- **Type:** Label style, 15px uppercase, 0.08em tracking. In a verdict, the code is 700 and the label 500.
- **Tones:** plain (Flap Slate, Flap White), ok (green on Green Cell), bad (red on Red Cell), run (amber on Amber Cell), wait (Label Grey on Row Slate).
- **Motion:** a 320ms flip (rotateX from -80deg on a top hinge, with a brief brightness flash, easing `cubic-bezier(0.16, 1, 0.3, 1)`). It plays only when the cell's state changes while the page is open, or when a cell arrives after load (a new test case). Cells present at load stand still. Reduced motion turns the flip off.
- **Lifecycle:** one cell follows a submission from IN QUEUE (wait) to JUDGING (run) to its verdict.

### Board Rows (lists)
Every list is a board: problems, contests, submissions, admin lists, contest problems and the leaderboard.
- **Row:** 48px, Row Slate, 2px gap between rows, 16px cell padding, the row name in the Title style. Linked rows step to Flap Slate on hover; link underlines are hidden until hover, then amber.
- **Results board:** each column is its own flap cell (case number, verdict, ms, KB), with the verdict flap filling its cell at row scale and figures right-aligned in the board face. Its rows have no links, so they do not light up.
- **Leaderboard:** rank in Label Grey, user, then 72px problem cells in the board face, filled green (solved), red (rejected) or amber (pending), and solved and penalty totals.

### Result Board
The submission's headline: a Board Black panel with a 1px Flap Slate border, a RESULT label in the board face, then the status flap and plain flaps for time and memory, all at 20px.

### Code Editor
Monaco with the "codearena-board" theme: Board Black ground, Flap White text, amber cursor, translucent amber selection, Board Ground current-line band, ramp greys for guides, whitespace and widgets. Syntax colouring is the one place colour appears outside state cells: muted amber keywords, soft green strings, soft orange numbers, soft blue types and grey italic comments. Keep these hues desaturated so they never compete with verdict cells.

## Do's and Don'ts

### Do:
- **Do** take every neutral from the seven-step ramp `--r0` to `--r6`.
- **Do** put every state (verdict, phase, limit, solved) in a split-flap cell with its code and label.
- **Do** size cells, controls and rows as 4u, 5u and 6u of the 8px module.
- **Do** set every list as a board of 48px rows with 2px gaps, one line per row, ellipsis on overflow.
- **Do** keep amber for progress, the single primary action and focus; one amber button per view.
- **Do** flip a flap only when its state changes or it arrives after load, and never under reduced motion.
- **Do** use Barlow Condensed uppercase for names of states, columns and sections; Barlow for sentences; JetBrains Mono only for code.

### Don't:
- **Don't** add greys outside the ramp, or use green or red outside a state cell, an error or a status confirmation.
- **Don't** use rounded pills or radii above 2px.
- **Don't** add drop shadows, blur, glows or decorative gradients; the hinged flap face is the only surface gradient.
- **Don't** animate cells on page load or add motion other than the flap flip and 160ms colour transitions.
- **Don't** let colour be the only signal of a verdict.
- **Don't** drift toward a dark IDE with green and red pills, a white SaaS dashboard, or a hacker terminal look.
