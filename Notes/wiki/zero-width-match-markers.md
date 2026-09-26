# Zero-width match markers

Issue #23
(`Notes/issues/023-zero-width-match-markers.md`, tasks
`Notes/tasks/023-zero-width-match-markers.md`) delivered the marker
cell: a zero-width submatch — `^`, `$`, an empty look-around — renders
as one inverse-video space at its mapped display location, underlined
on the current matched line, participating in every shared rule
(reveal, wrap, clip, extent, indicators) like any other match cell.

PRD cross-references: "Text, graphemes, and safe presentation" (the
zero-width bullets — the one-cell marker, the effective-width
extension, cluster-start mapping, and the terminator-only marker's
ordinary-rule status) in `Notes/PRD-vrg.md`, plus "Testing
Decisions → FileBuffer / Viewport".

## The marker cell

A `present.Span` with `Start == End` is a **marker position**, not a
coverage range — the shape `Line.Span` has produced since Issue #22
for zero-width positions and removed terminator bytes, and what
`filebuffer.Load` emits for a validated empty submatch
([line-terminators-and-bom.md](line-terminators-and-bom.md)).
`clusterSpan` passes markers through unchanged — they are already
cell-precise
([grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)).

`renderCells` (`internal/app/browse.go`) paints a marker as **one
inverse space at its cell without shifting following text**: on an
existing cell it replaces the glyph — a marked wide cluster's trailing
cell paints blank beside it so the row's width accounting holds and
the glyph is never split — and at end of line it extends the line by
one inverse cell. The style is `Theme.Match`, upgraded to
`Theme.CurrentMatch` (inverse + underline) on the current matched
line, exactly like coverage spans.

## Mapping

The marker's cell comes straight out of Issue #22's byte→cell map:

- **A position inside a cluster maps to the cluster's start cell** —
  `lo[byte]` is always the unit's first cell, so a recorded position
  on an interior byte of a wide pair or a ZWJ sequence marks the glyph
  whole rather than splitting it.
- **A position at or past the line's end maps to the display
  end-of-line position** — including positions on removed LF/CRLF
  terminator bytes, which retain their raw slots pointing there.
- **An empty matched line maps to cell 0**, giving it an effective
  width of one.

## One cell past the text

Because the marker can sit at `len(cells)` — one past the last real
cell — each downstream layer carries the extension:

- **Wrap**: `wrapLine` packs only real cells; when a line's final row
  is completely full, an end-of-line marker occupies **another row** —
  an empty continuation row painting the marker at column zero
  ([wrap-mode.md](wrap-mode.md)). `RowOf` resolves the marker cell to
  that row, so the reveal target lands correctly.
- **Clip**: `clipRow` keeps a marker whose position falls inside the
  window, translated to window cells; a marker at the position just
  past the window's right edge is clipped away
  ([horizontal-panning.md](horizontal-panning.md)).
- **Reveal**: `revealCell` treats a marker target as a one-cell unit
  via `markedAt` — no cluster whole needed — so an end-of-line marker
  reveals to `marker + 1 − width` and a marker on a clipped cluster's
  lead cell counts as painted
  ([minimal-horizontal-reveal.md](minimal-horizontal-reveal.md)).
- **Extent**: `lineExtent` joins markers into both values — an
  end-of-line marker extends the content extent by one cell (a
  marker-only line has extent 1) and each marker is itself a paintable
  boundary, so the Issue #18 maximum can land the window's first
  column on the marker — even past a final cluster too wide to fit
  ([horizontal-panning.md](horizontal-panning.md)).
- **Indicators**: `hiddenMarks` counts a marker as one painted cell
  wherever it sits — even on a split cluster's blanked cell — so only
  positions outside the window are hidden; an entirely hidden marker
  upgrades the gutter signpost to `*` and feeds the reserved right
  column's `*` on the current matched line, a marker-only line's
  hidden marker sufficing though it has no text to hide
  ([hidden-content-indicators.md](hidden-content-indicators.md)).

## The terminator-only marker is ordinary

A match solely on removed terminator bytes — `$` on `hit\r\n`, whose
position maps to display column 3 — produces a single marker cell
there and is **not a special case**: it follows exactly the same
reveal, wrap, clip, horizontal-extent, and indicator rules as any
other marker, including counting as "entirely hidden" for the gutter
`*` and the right `*`. `marker_test.go`'s
`TestTerminatorMarkerIsOrdinary` exercises all five surfaces on the
one fixture.

## Tests

- `internal/filebuffer/marker_test.go` — the marker positions through
  `Load`: `^`/`$`-style empty submatches at BOL (text and empty
  lines), mid-line, at EOL, and on every line of a multi-line file;
  positions inside a wide pair and a ZWJ cluster mapping to the
  cluster start; and LF/CRLF terminator positions landing on display
  column 3 of `hit\r\n`.
- `internal/viewport/marker_test.go` — marker participation: the EOL
  marker extending extent so the pan maximum lands on its cell (past
  text and past an unfittable final cluster), the marker-only line's
  extent 1 with maximum offset 0, hidden markers driving the gutter
  and right `*` (a marker-only line flags `*` with no text hidden),
  the `hit\r\n` marker's ordinary reveal/clip/wrap/extent/indicator
  behavior, and interior markers following their cell through
  clipping with hidden-side flags.
- Existing coverage predating the issue — `wrap_test.go`'s
  `TestEndOfLineMarkerRows`, `hreveal_test.go`'s
  `TestHRevealMarkerCells`, `indicators_test.go`'s marker rows, and
  `app/cluster_test.go`'s marker-on-blank paint — now exercises the
  finished contract.

See [unit-tests.md](unit-tests.md) § `internal/filebuffer` and
`internal/viewport`.

## Files

- `internal/viewport/viewport.go` — `lineExtent` counts marker spans:
  each is a one-cell unit extending the extent at line end and a
  paintable boundary at its position. The remaining marker machinery
  predates the issue: `markedAt`/`revealCell`, `clipRow`'s marker
  translation, and `hiddenMarks`' marker visibility.
- `internal/viewport/rows.go` — `wrapLine`'s end-of-line marker
  overflow row and `Row`'s marker span translation were laid down with
  Issue #16–#20's models.
- `internal/app/browse.go` — `renderCells` paints the marker's inverse
  space in place or appended at line end; `reveal` already targets a
  marker's `Start` cell.
- `internal/filebuffer/filebuffer.go` — `Load` validates empty
  submatches against the raw bytes and `clusterSpan` passes their
  positions through.

See also:
[line-terminators-and-bom.md](line-terminators-and-bom.md) (the
byte→cell mapping the marker position comes from),
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the passthrough and the no-split-glyph rule),
[safe-presentation.md](safe-presentation.md) (the `Span` marker
shape), [destination-reveal.md](destination-reveal.md) (the marker
cell as the navigation target), and
[theme-and-colour-toggle.md](theme-and-colour-toggle.md) (the
`Match`/`CurrentMatch` styles it paints in).
