# Standalone combining cluster fallback cell (Issue #43)

Issue #43 materializes a real one-cell display fallback for standalone
combining clusters — grapheme clusters with no base and no independent
visible cell (`Cluster.Width == 0`, e.g. a combining mark at line start
or after a grapheme-breaking zero-width space). Before this issue,
`expandedByteCells`/`expandedHighlights` only annotated such a cluster
with a synthetic `[start, start+1)` cell range while `cellPos` still
advanced by the original zero width: a following cluster could overlap
the supposed fallback, and the Issue #39 cluster-driven renderer had no
actual independent cell to paint (the mark rendered unstyled while the
next character took its highlight).

Depends on [Issue #39](../issues/039-render-from-shared-grapheme-cell-model.md)
(the cluster-based renderer the fallback propagates through). This page
cross-references:

- [grapheme-cluster-highlight-expansion](grapheme-cluster-highlight-expansion.md)
  — the Issue #21 expansion machinery the fallback feeds.
- [shared-cell-model-render](shared-cell-model-render.md) — the final
  render path that paints the fallback cell.
- [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md) —
  the shared `Cluster`/`GraphemeClusters` policy.
- `Notes/issues/043-combining-cluster-fallback-cell.md` — the issue.
- `Notes/decisions/043-combining-cluster-fallback-cell.md` — the
  recorded representation decision (HITL gate).
- `Notes/PRD-vrg.md` — *Text, graphemes, and safe presentation*.

## Recorded representation: dotted-circle base

The issue required one deterministic human-chosen representation.
Selected: **Candidate A** — display `U+25CC ◌` (DOTTED CIRCLE, UTF-8
`E2 97 8C`) immediately before the cluster's original combining-mark
bytes, so the marks compose onto the conventional "no base" carrier and
remain visually identifiable. Rejected: space base (marks compose onto
an invisible carrier, less consistent) and fixed placeholder (discards
the marks' identity and conflates with the invalid-UTF-8 `U+FFFD`).

Under the shared `rivo/uniseg` policy, `◌` + combining marks segments
as a single grapheme cluster (no break before extending characters) of
width 1 — the expected result is exactly one cluster occupying exactly
one terminal cell.

## Normalization rule

The fallback cell is *constructed* as one cell rather than re-measured:
`filebuffer.standaloneClusterFallback` records the fallback unit
(`◌` + the original cluster's bytes) as a single cluster with `Width`
pinned to 1 in the line's cluster table. A width-library report of 0 or
>1 for a pathological mark sequence, or a terminal that renders the
composed unit inconsistently, cannot change the recorded cell geometry.
The defensive `max(c.Width, 1)` annotation inside `expandedByteCells`
and `expandedHighlights` remains, but `cellPos` now advances by the
effective width so even a residual zero-width cluster could not be
overlapped; `Load` guarantees none survive.

## Byte-mapping contract

Byte-to-cell mapping still resolves the fallback cell to the cluster's
original source bytes: every source byte of the former zero-width
cluster maps to the fallback cell's `[cell, cell+1)` range. The inserted
`◌` bytes are display-only — they carry no source-byte mapping, and
`ByteOffsets` shift by the three inserted bytes at and after each
insertion point so every later source byte still lands inside its own
cluster. Match/highlight mapping, stale validation, and raw-byte
coordinates are unaffected.

## One-cell propagation

`standaloneClusterFallback(text, byteOffsets, clusters)` runs in
`filebuffer.Load` between `GraphemeClusters` and the Issue #21
expansion, rewriting the display text, `ByteOffsets`, and the cluster
table together. From that point every layer sees an ordinary width-1
cluster carrying real display bytes:

- `expandedByteCells` / `expandedHighlights`: `cellPos` advances by the
  real width — the following cluster's cells no longer share the
  fallback cell; a match on the mark highlights exactly `[cell, cell+1)`.
- `Line.ContentWidth` (`clusterContentWidth`) counts the fallback cell,
  so end-of-line positions, markers, and reveal targets sit after it.
- Wrapping (`viewport.wrapLine`), clipping
  (`viewport.clipLineToWindow`), and horizontal panning extents count
  the fallback like any other cell — it can be hidden left/right and
  reported by the Issue #20 indicators.
- `renderLineWithHighlights` (Issue #39) paints the `◌`+marks bytes as
  a real cell; a covering match styles exactly the fallback cell and
  nothing adjacent.

## Tests

See [unit-tests](unit-tests.md): `filebuffer/cluster_fallback_test.go`
(display bytes, ByteCells, content width, highlight spans,
multiple-fallback and one-cell normalization) and
`app/fallback_cell_test.go` (composed-view painted bytes, wrap, and
pan/clip counting).
