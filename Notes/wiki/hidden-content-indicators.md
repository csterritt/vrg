# Hidden-content indicators

Issue #20
(`Notes/issues/020-hidden-content-indicators.md`, tasks
`Notes/tasks/020-hidden-content-indicators.md`) delivered the
hidden-content indicators: in run-off-edge mode every visible source
line's gutter signposts content hidden left of the window, and a
reserved rightmost column signposts a match hidden right — both in the
theme's inverse indicator style, computed from the actually rendered
cells.

PRD cross-references: "Layout and indicators" (the `_`/`*` gutter
bullet, the reserved-column right-star bullet, and the
visibility-over-rendered-cells rule) in `Notes/PRD-vrg.md`.

## The gutter signpost

The first trailing gutter space of **every visible source line** — the
cell between the right-justified line number and the final separator
space — is painted by `contentRow` (`internal/app/browse.go`) as:

- **blank** when nothing on the line is hidden left;
- an **inverse `_`** (`Theme.Indicator`) when any of the line's *text*
  is hidden left — including a line wholly left of the window, and on
  every line at once when a nonzero offset leaves them uniformly so
  ([horizontal-panning.md](horizontal-panning.md) legalized that
  geometry);
- an **inverse `*`** — the upgrade — when a match *or marker* on that
  line is entirely hidden left. A hidden marker alone suffices even on
  an otherwise empty line; a match only *partially* hidden stays `_`.

Wrap-mode rows never carry the flags, so the gutter there keeps its
two plain trailing spaces — continuation rows included.

## The reserved right column

Run-off-edge mode reserves the file panel's rightmost column —
`reservedW()`'s `1` since Issue #16
([wrap-mode.md](wrap-mode.md)). It is blank on every row except the
**current matched line's** visible row, where an inverse `*` appears
when a match or marker on that line is entirely hidden right. Two
scope asymmetries are deliberate:

- **Current line only.** A non-current matched line — or an unmatched
  one — with a hidden-right match draws nothing in the column; and
  there is no right-side equivalent of the `_` text signpost.
- **Off-screen absence.** When the current matched line scrolls
  vertically out of view no right `*` is drawn anywhere, while the
  other lines' gutter indicators keep signposting.

The column never overwrites text: `contentRow` pads the painted cells
to the text width and emits the indicator as the cell after them, so a
match ending on the last text cell still paints its final glyph beside
a `*` earned by a farther match. In wrap mode `reservedW()` is 0 — no
column exists and nothing is appended.

## Visibility basis

The flags live on `viewport.Row` as `HiddenLeft`, `MatchHiddenLeft`,
and `MatchHiddenRight`, computed by `hiddenMarks` inside `clipRow` —
which already knows the painted window `[off, off + text width)`, the
clip edge blanking, and the unclipped line's spans that the clipped
`Row` drops. Visibility is **painted-cell visibility**, the Issue #19
rule the indicators share
([minimal-horizontal-reveal.md](minimal-horizontal-reveal.md)):

- The reserved column is excluded automatically — the window's `w` is
  the text width.
- A **marker** is one painted cell wherever its position sits — even
  on a split cluster's blanked lead — so only positions outside the
  window are hidden.
- A **coverage match** is visible when at least one of its cells
  paints; a *partially* painted match counts as visible for that side
  entirely.
- A cluster split by a clip edge renders its in-window cells blank, so
  a match on it is *not* visible — a right-edge split reports hidden
  right, a left-edge split hidden left, and the Issue #19
  unpaintable-cluster case (blank at every offset) reports hidden
  right from its start column.
- An entirely hidden match is attributed to the side its hidden cells
  stand on — both flags only when a blank-filling cluster straddles
  both edges — so a line can carry the gutter `*` and the column `*`
  together.

Since Issue #21 the spans classified here are the cluster-expanded
spans `Buffer.Spans` hands down — a match recorded mid-cluster counts
from its cluster start
([grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)).

## Tests

- `internal/viewport/indicators_test.go` — the `Row` flags: text
  hidden left (offset zero, empty line, fully-hidden line); match and
  marker entirely hidden on each side with half-painted cases counting
  visible; both sides at once; split-cluster blanks counting as hidden
  in each direction; the unpaintable cluster reporting hidden right;
  and wrap models carrying no flags.
- `internal/app/indicators_test.go` — the rendered frame:
  `_`/`*`/blank gutters per line including the empty line and the
  uniform-lines fixture, the inverse SGR pair, the current-line-only
  right `*` with non-current and unmatched lines blank, its absence
  when the current line scrolls off-screen, both stars together,
  partial visibility drawing no star on either side, the last-text-cell
  match painted beside a farther match's `*`, split-glyph blanks
  producing stars, and wrap mode drawing no indicators and no reserved
  column.

See [unit-tests.md](unit-tests.md) § `internal/viewport` and
`internal/app`.

## Files

- `internal/viewport/viewport.go` — `Row` gained
  `HiddenLeft`/`MatchHiddenLeft`/`MatchHiddenRight`; `clipRow` calls
  `hiddenMarks`, which resolves the blanked regions
  `[off, lb)`/`[rb, off+w)` the same way `clipRow` blanks them and
  classifies each span by painted cells.
- `internal/app/browse.go` — `contentRow` composes the gutter as
  number + indicator cell + separator space and appends the padded
  reserved column's `*`/` `; `reservedW` now backs real content.

See also: [horizontal-panning.md](horizontal-panning.md) (the offset
and clipping these flags describe),
[minimal-horizontal-reveal.md](minimal-horizontal-reveal.md) (the
shared painted-cell visibility rule),
[wrap-mode.md](wrap-mode.md) (the indicator-free mode and the column's
reservation), [browse-tracer.md](browse-tracer.md) (the gutter and
match styling the indicators join),
[theme-and-colour-toggle.md](theme-and-colour-toggle.md) (the inverse
`Indicator` style),
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the expanded spans the flags classify), and
[safe-presentation.md](safe-presentation.md)
(the shared grapheme policy).
