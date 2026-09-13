# Zero-width match markers

Issue #23: zero-width regex submatches (`Start == End`) render as
navigable, inverse-video marker cells that obey the project's existing
grapheme, wrapping, clipping, reveal, extent, panning, and
hidden-content-indicator contracts. Cross-reference
[Issue #23](../issues/023-zero-width-match-markers.md) and the
[Text, graphemes, and safe presentation](../PRD-vrg.md) section of the
PRD.

## Marker representation

A zero-width submatch renders as exactly one inverse-video space at
its mapped display location. The marker is a one-cell highlight range
`[cell, cell+1)` appended to `Line.Highlights` by `filebuffer.Load`,
coexisting with the non-zero-width highlights from Issue #21. On the
current matched line the marker is underlined (the App applies
`theme.CurrentMatch` to the current line and `theme.Match` elsewhere,
so the marker inherits the same styling as any other highlight).

The marker marks an existing cell without shifting following text. A
marker in the middle of a line (not at end of line) does not change
`Line.Display` or extend the cluster width; it only adds the one-cell
highlight over the existing cell.

## Cluster-start mapping

A zero-width position inside a grapheme cluster maps to the cluster
start cell, so wide glyphs are never split. `filebuffer.Load` derives
the marker cell from the expanded `ByteCells` (Issue #21): a byte
inside a cluster maps to the cluster's full cell range, so
`ByteCells[sm.Start][0]` is the cluster start cell. A zero-width
position at the start of a cluster likewise maps to that cluster's
start cell.

## End-of-line extension

A marker at end of line extends the effective line width by one cell.
`filebuffer.Load` appends a single space to `Line.Display` and a
1-cell `safepresentation.Cluster` to `Line.Clusters` so the marker has
a paintable cell. An empty matched line therefore has width one: its
`Display` is a single space and its `Clusters` slice has one 1-cell
cluster. A marker after a completely full wrap row occupies another
row because the extra cluster participates in the Issue #16 wrap model.

The EOL marker cell is the last cluster in `Line.Clusters` and has
width 1. Multiple zero-width submatches at the same EOL position
produce one marker (duplicate cells are deduplicated).

## Terminator-only markers

A marker solely on removed terminator bytes — such as a `$` match on
`hit\r\n` at the `\n` byte (byte 4) — maps to display column 3 (the
end-of-line column from Issue #22). The terminator-only marker is an
ordinary end-of-line marker with no special-case rendering path: it
follows exactly the same reveal, wrap, clip, horizontal-extent, and
indicator rules as any other marker, including counting as entirely
hidden for the Issue #20 gutter `*` and right reserved-column `*`
indicators.

## Wrap and clip

The EOL marker's virtual cluster participates in the Issue #16 wrap
model. A line that exactly fills the text width has its EOL marker on
the next wrap row; a line that does not fill the text width keeps the
marker on the same row. The marker cell participates in the Issue #18
clipping: a marker hidden left of the clip window is excluded from the
clipped output, and a marker visible in the window is included with
its highlight shifted to local coordinates.

## Horizontal extent and pan clamping

The marker cell participates in horizontal extent and pan clamping.
`widestLine` and `paintableMaxOffset` (Issue #18) inspect `Clusters`,
so a line whose extent includes an EOL marker has its extent and
maximum offset computed including the marker cell. A marker-only line
(an empty matched line) contributes extent 1 and has maximum
horizontal offset 0 under Issue #18's paintable-boundary maximum: the
single marker cluster starts at cell 0 and its width (1) fits any text
width ≥ 1, so the maximum offset is 0.

A line with content plus an EOL marker has its maximum offset include
the marker cell. For example `"hit"` (3 cells) plus an EOL marker (1
cell) at text width 3 has clusters `[h, i, t, marker]`; the marker
cluster starts at cell 3 and its width (1) fits the text width, so
the maximum offset is 3.

## Reveal targeting

Marker cells are navigable reveal targets. The Issue #19
`RevealHorizontal` arithmetic uses `ClusterWidthAtCell`, which returns
1 for the marker cell, so the reveal works like any single-cell
target. The Issue #14 vertical reveal targets the first submatch's
start byte; a zero-width submatch at end of line maps through
`ByteCells[sm.Start][0]` to the EOL display column, so the reveal
targets the marker cell.

## Indicator participation

Markers participate in the Issue #20 hidden-content indicators. A
marker entirely hidden left produces a gutter `*`; a marker entirely
hidden right produces a right `*` on the current matched line's visible
row; a visible marker produces no indicator. The marker highlight is
a one-cell range consumed by the same `highlightHasVisibleCells` and
`hasHiddenMatchRight` logic as any other highlight, so the
terminator-only `$` marker follows the same indicator rules as any
other marker.

## Implementation

`filebuffer.Load` produces the markers:

- `markerCellsForStops(stops, byteCells, clusters)` returns the display
  cell positions of zero-width submatches, mapping each through the
  expanded `ByteCells` (cluster-start mapping) and deduplicating.
- `clusterContentWidth(clusters)` returns the sum of cluster widths,
  the display content extent before any EOL marker extension.
- For each marker cell equal to the content width (EOL), `Load`
  appends a space to `Display` and a 1-cell cluster to `Clusters`.
- Each marker cell is appended to `Highlights` as `[cell, cell+1)`.
- `Highlights` are sorted by start cell so `renderLineWithHighlights`
  processes them in cell order.

The Viewport needs no marker-specific changes: the existing
cluster-driven wrap model, clipping, extent, pan clamping, and reveal
arithmetic operate on the marker's virtual cluster and one-cell
highlight like any other cluster and highlight. The App's
`renderLineWithHighlights` paints the marker space with the match or
current-match style, and the indicator functions consume the
one-cell highlight like any other.

## Sources

- `Notes/issues/023-zero-width-match-markers.md`
- `Notes/PRD-vrg.md` (Text, graphemes, and safe presentation)
- `internal/filebuffer/filebuffer.go`
- `internal/filebuffer/marker_test.go`
- `internal/viewport/marker_test.go`
- `internal/app/marker_indicator_test.go`

See also
[grapheme-cluster-highlight-expansion](grapheme-cluster-highlight-expansion.md),
[structural-line-handling](structural-line-handling.md),
[wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md),
[horizontal-panning](horizontal-panning.md),
[horizontal-reveal](horizontal-reveal.md),
[hidden-content-indicators](hidden-content-indicators.md),
and [destination-reveal](destination-reveal.md).
