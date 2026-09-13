# Hidden-content indicators (Issue #20)

Issue #20 populates the run-off-edge hidden-content indicators whose
column was reserved by Issue #16 and whose horizontal clipping and
visibility data come from Issues #18 and #19. In run-off-edge mode, the
first trailing gutter space carries a left indicator and the reserved
rightmost column carries a right indicator. Wrap mode draws neither
and reserves no right column.

This page cross-references:

- [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md) —
  the reserved indicator width (`ReservedWidth`) that Issue #20
  populates.
- [horizontal-panning](horizontal-panning.md) — the horizontal offset,
  grapheme-safe clipping, and the visible window `[hOffset,
  hOffset+textWidth)` that the indicators describe.
- [horizontal-reveal](horizontal-reveal.md) — the painted-cell
  visibility definition (split clusters are not painted) that the
  indicator visibility computation reuses.
- [theme-module](theme-module.md) — the `Theme.Indicator` inverse
  style used for both indicators.
- `Notes/PRD-vrg.md` — Layout and indicators section.

## Left gutter indicator

Every visible source line's first trailing gutter space (the first of
the two spaces after the right-justified line number) carries the left
indicator in run-off-edge mode:

- inverse `_` when any text is hidden to the left of the visible
  window;
- upgraded to inverse `*` when a match or marker on that line is
  entirely hidden to the left;
- blank otherwise (no text hidden left, or the line has no
  non-zero-width content).

The renderer emits the line number, the indicator, then the second
trailing space, then the clipped text. Continuation rows keep a blank
gutter (no indicator); they only arise in wrap mode, which has no
indicators.

## Right reserved-column indicator

The reserved rightmost column (one cell, already excluded from the
text width by `ReservedWidth(WrapOff) == 1`) carries the right
indicator:

- inverse `*` on the current matched line's visible row when at least
  one match or marker on that line is entirely hidden to the right;
- blank on every other row, and blank on the current matched line when
  no match is entirely hidden right.

The right indicator never overwrites text: the text width already
excludes the reserved column, so the renderer pads the clipped text to
the text width before writing the reserved column. Both left and
right stars may appear together on the same row.

## Off-screen current line

If the current matched line is vertically off-screen, its right
indicator is absent — there is no visible row to carry it. Other
lines' left gutter indicators remain, because every visible source
line carries its own left indicator independent of the current match.

## Partial visibility

A partially visible match produces no hidden-match indicator for that
side. A match straddling the left clip edge with at least one
non-blank rendered cell in the window is not "entirely hidden left",
so the left indicator stays `_` (text hidden) rather than `*` (match
hidden). Symmetrically for the right edge.

## Visibility basis: rendered cells after clipping

Visibility is calculated over the actually rendered text/marker cells
after grapheme clipping, not over raw byte or rune positions:

- The reserved right column is excluded from visibility calculations
  (it is not part of the text window).
- A split wide glyph rendered only as blanks (its in-window portion
  after clipping) does not count as a visible match. A match whose
  only in-window cells are blank because the underlying cluster was
  split by a clip edge is treated as entirely hidden for that side.

The computation walks the source line's grapheme clusters and tests
whether any fully visible (non-split) cluster overlaps the highlight
range within the window. A cluster split by either clip edge
contributes no non-blank cells and so cannot make a match visible.

## Wrap mode

Wrap mode draws neither hidden-content indicators nor a reserved
right column. `ReservedWidth(WrapOn) == 0`, so the text width is the
full panel width minus the gutter, and the renderer uses the plain
two-space trailing gutter with no left indicator and no right column.

## Implementation

`renderContentPanel` (in `internal/app/app.go`) computes the
indicators in run-off-edge mode only:

- `leftIndicator(line, theme, hOffset, textWidth)` returns the styled
  left gutter indicator: `*` if any highlight is entirely hidden left
  (no non-blank visible cells and starts before the window), `_` if
  text is hidden left, or a blank otherwise.
- `hasHiddenMatchRight(line, hOffset, textWidth)` reports whether any
  highlight is entirely hidden right (no non-blank visible cells and
  ends past the window). The renderer emits `theme.Indicator("*")` in
  the reserved column only on the current matched line's visible row.
- `highlightHasVisibleCells(hl, clusters, hOffset, windowEnd)` walks
  the clusters and reports whether any fully visible (non-split)
  cluster overlaps the highlight's in-window cell range. Split
  clusters (rendered as blanks) do not count.
- `clusterCellWidth(clusters)` sums cluster widths so the renderer
  pads the text area to the text width before the reserved column,
  guaranteeing the right indicator never overwrites text.

Both indicators use `Theme.Indicator`, the inverse match colour pair
restored to the base colours. With the no-style theme, the indicator
characters appear without ANSI sequences for sink-safety testing.

## Tests

`internal/app/indicator_test.go` covers the Issue #20 contracts
through observable Bubble Tea `Update` and rendered output:

- left `_` for hidden text without a hidden match;
- left `*` for a match entirely hidden left;
- right `*` on the current matched line when a match is entirely
  hidden right;
- right indicator absent when the current matched line is
  off-screen, while other lines' left indicators remain;
- both left and right `*` together on the same row;
- partial visibility on either side produces no hidden-match
  indicator for that side;
- a visible last-cell match with a farther-right hidden match
  produces a right `*` and a left `_`;
- a split wide glyph rendered as blanks does not count as visible
  match content (left `*`);
- wrap mode draws neither indicators nor a reserved right column;
- indicator styling uses the theme's inverse indicator style for
  both `*` and `_`.
