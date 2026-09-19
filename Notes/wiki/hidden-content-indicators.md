# Hidden-content indicators — gutter `_`/`*` and the reserved right `*` (Issue #20)

Delivered by
[Issue #20](../issues/020-hidden-content-indicators.md)
([task](../tasks/020-hidden-content-indicators.md)): in run-off-edge
mode the file panel signposts content the horizontal window cannot
show — a per-line gutter mark for the left edge and a reserved
right-column star scoped to the current matched line. Wrap mode draws
neither and reserves no column. Relevant PRD sections: *Layout and
indicators* (the indicator bullets and the deliberately asymmetric
scope) and *Navigation, viewport, and logical anchors* (visibility
over rendered cells) in [`Notes/PRD-vrg.md`](../PRD-vrg.md); user
stories 67–69. Builds on
[horizontal-reveal.md](horizontal-reveal.md) (the `CellVisible`
painted-cell visibility the indicators share),
[horizontal-panning.md](horizontal-panning.md) (the `off` window the
marks describe), [wrap-mode.md](wrap-mode.md) (the reserved 0/1
indicator width the right star populates), and
[theme.md](theme.md) (`Theme.Indicator`, the inverse style both
marks use).

## The visibility basis — rendered cells after clipping

Every judgment derives from the painted-cell visibility Issue #19
exposed: a cell counts visible only when the whole grapheme cluster
holding it fits inside `[off, off + textW)` — the reserved indicator
column excluded, since the width is the text width. Consequences the
indicators inherit directly:

- A cluster straddling either edge renders its in-window cells as
  clipping blanks, so a **geometrically inside position can still be
  hidden** — a split wide glyph rendered only as blanks is not a
  visible match.
- A match straddling an edge is **partially visible** and earns no
  hidden-match indicator for that side: the star requires a match
  *entirely* hidden.
- Zero-width and end-of-line marker cells
  ([Issue #23](zero-width-markers.md)) count as one-cell targets —
  an empty highlight span is judged at its single marker position, so
  a hidden marker upgrades the marks exactly like a hidden match.
- The unpaintable-cluster geometric fallback relaxes only the reveal:
  an oversized match's clipped blanks still count as hidden, so the
  indicators keep reporting it.

## The left gutter mark — `viewport.LeftMark` (per line)

Every visible source line uses the first trailing gutter space for an
indicator, styled by `Theme.Indicator`:

- `*` when a match or marker on the line is **entirely hidden left**
  (every span cell's cluster starts before `off`),
- `_` when text is hidden left but no match is entirely hidden
  there,
- blank otherwise — an empty line included, which has no text to hide.

The scope is every visible source line, current or not — deliberately
asymmetric with the right star. A uniform-lines window where every
visible line has text hidden left therefore marks every row, which is
permitted: the pan clamp's painted-cluster guarantee bounds painted
cells, not indicator counts.

## The reserved right column — `viewport.RightMark` (current line only)

The run-off-edge layout reserves one rightmost column — carved out of
the text width, so it **never overwrites text**, and the last text
cell can hold a painted match while the star reports a farther match
beyond it. The column is blank except on the current matched line's
visible row, where `*` means at least one match or marker is entirely
hidden right (every span cell's cluster ends beyond `off + textW`).

The star follows the cursor: `n`/`p` moves it to the newly current
line's row, a non-current line with a hidden-right match shows
nothing, and when the current matched line is vertically off-screen
its right indicator is absent while other lines' gutter marks remain.
Left and right stars may appear together — one match entirely hidden
left, another entirely hidden right.

## The render path — `contentCell`

`contentCell` composes the row as gutter + text + reserved column. In
run-off-edge mode it asks `LeftMark`/`RightMark` for the row's marks
(gated by `!m.wrap`, the right star additionally by the row being the
current matched line and the column existing), substitutes the left
mark for the gutter's first trailing space inside a three-part
`Gutter`/`Indicator`/`Gutter` composition, and renders the right
`*` through `Theme.Indicator` or leaves the reserved cell blank. Wrap
mode keeps both trailing gutter spaces and the text area claims the
column the flat layout reserves.

## Tests

`internal/viewport/indicators_test.go` (external package): the
`LeftMark` and `RightMark` tables — entirely/partially hidden spans on
both edges, clipped-grapheme blanks counting hidden (`文` split by the
window edge upgrades the mark), painted targets, empty-row and
zero-width-marker one-cell positions, the last-cell-match case, and
`_` on every row of a uniform-lines model at a nonzero offset.

`internal/app/indicators_test.go` (same package) drives the render
under `theme.Plain()` and the dark scheme: `_` versus `*` gutters per
visible line including the empty-line blank and the all-hidden `_`;
the right star following `n` between matched lines and absent when the
current line scrolls off-screen; both stars together; partial
visibility suppressing each side's star; the last-cell match painted
beside a farther match's star; split-glyph blanks upgrading to `*` at
both edges; a mark on every row of a uniform-lines window; wrap mode
drawing neither mark nor column; and the marks' inverse SGR styling.
`pan_test.go` and `hreveal_test.go` row expectations gain the marks at
nonzero offsets — the unpaintable-tab row now shows the gutter `_`
plus the reserved `*`. See [unit-tests.md](unit-tests.md).
