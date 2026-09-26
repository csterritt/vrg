# Standalone combining-cluster fallback cell

Issue #43 (task
`Notes/tasks/043-combining-cluster-fallback-cell.md`) generalized the
line-start provisional fallback into a rule for every standalone
zero-width grapheme cluster: a cluster the shared grapheme policy
reports as its own unit but with no base and no independent visible
cell paints one real terminal cell — **U+25CC DOTTED CIRCLE followed
by the cluster's original bytes** — so it is always visible,
highlightable, and reachable instead of merging invisibly into
whatever precedes it.

PRD cross-references: "Text, graphemes, and safe presentation" (the
grapheme-expansion bullets, which record the fallback contract) in
`Notes/PRD-vrg.md`. The representation is the recorded decision in
`Notes/decisions/043-combining-cluster-fallback-cell.md` — Option A
chosen over a space base (visually indistinguishable from an ordinary
space-plus-marks cluster) and over a fixed replacement glyph (which
would discard the source's marks).

## Representation and normalization

The fallback cell's text is the UTF-8 encoding of U+25CC
(`EF 97 8C`) followed by the standalone cluster's source bytes
verbatim: a bare `U+0301` paints `EF 97 8C CC 81` (`◌◌́`), and a
standalone `U+0301 U+0302` cluster paints `EF 97 8C CC 81 CC 82`.
Under the shared policy the sequence is one grapheme cluster (U+25CC
is an ordinary base followed by Extend runes) of measured width 1 —
but the one-cell occupancy is **structural, not measured**: `emit`
appends exactly one `Cell`, never a lead-plus-continuation pair and
never a count from a width call, so a width library returning 0 or >1
cannot skew `Line.Width`, spans, wrapping, or horizontal extents.

The `◌` prefix is display-only — it occupies no byte→cell mapping
slot. The cluster's original source bytes map to the fallback cell
through the `lo`/`hi` tables exactly as before, so `Line.Span` over
those bytes yields the fallback's one-cell range, a match on them
highlights exactly that cell, and a zero-width position inside the
cluster marks its index.

## Which units take the fallback

The fallback applies to a zero-width unit that **is** a new grapheme
cluster — never to a zero-width rune inside one. `lineOf` now keeps
that distinction at the segmentation layer: a printable ASCII byte
followed by a non-ASCII byte takes the cluster path rather than the
per-byte fast path, so `"e"` + U+0301 emits as the single cluster
`uniseg` reports (`"e◌́"`, width 1) and no mark can be orphaned from a
printable predecessor by the byte-at-a-time split. What remains as a
`lead` zero-width emit is then standalone by construction — at line
start or after a break-eligible predecessor (a caret escape like
`^A`, a tab expansion, a `^M`, or another standalone cluster) — and
takes the `◌` cell.

Non-standalone zero-width units still compose onto a host:

- marks inside a normal base cluster (`e◌́`, a mark after a space —
  the space is a real base, `" ◌́"` is one cluster) merge into the
  cluster's last cell;
- a mark the policy groups with an invalid byte (`"\xff◌́"` is one
  cluster) composes onto the U+FFFD cell — the eager `\ufffd`
  fast-path emit was removed so the byte flows through its real
  cluster;
- marks inside a printable/dangerous mixed cluster merge onto their
  cluster's preceding cell via the non-`lead` per-rune path.

Standalone non-mark clusters take the same fallback: a mid-line
U+FEFF paints `◌\ufeff` rather than vanishing into the previous cell,
and a zero-width U+2028 separator after a wide glyph becomes its own
`◌\u2028` cell. Two adjacent standalone clusters get two cells —
`x\u2028◌́` paints `x`, `◌\u2028`, `◌◌́`.

## One-cell propagation downstream

Because the fallback is an ordinary `Lead` cell in `Line.Cells`,
nothing downstream special-cases it:

- **`internal/present`** — `Line.Text` includes the `◌`-prefixed
  bytes, `Line.Width` counts the cell, and `Line.Span` maps the
  cluster's raw bytes to it.
- **`internal/filebuffer`** — `Load`/`Prepare` carry the cell through
  `Buffer.Cells`/`Text`; validated submatch bytes map to the fallback
  cell, and `clusterSpan` boundary expansion finds nothing to grow
  into — a mark-only match stays exactly the fallback cell, never
  borrowing the preceding escape's cells as it did under the old
  merge behaviour.
- **`internal/viewport`** — wrap rows and run-off-edge `clipRow` see
  one ordinary cell: the fallback wraps on its own boundary, clips at
  the window edge like any lead cell, and counts in horizontal pan
  extents.
- **`internal/app`** — `renderCells` paints the fallback cell with
  match styling when a span covers it; horizontal pan, minimal reveal
  arithmetic, hidden-content indicators, and async reveal offsets all
  treat it as a real painted cell (the pre-#43 tests that expected a
  borrowed-cell span were rewritten to the fallback geometry: a match
  on the mark in `a\x01◌́x` is `[3,4)`, not the `^A` cells).

## Tests

- `internal/filebuffer/fallback_test.go` — `TestStandaloneFallbackCells`
  (mark after a caret escape, after a tab expansion, a zero-width
  separator after a wide glyph, multi-mark and adjacent-cluster
  cases, the invalid-byte exception), `TestStandaloneFallbackByteMapping`
  (the cell resolves to original source bytes; the next cluster is the
  next cell), and `TestStandaloneFallbackSpanIsOneCell` (a mark match
  highlights exactly the fallback cell).
- `internal/present/line_test.go` — standalone-mark text/width/span
  cases and the `e◌́` base-cluster regression pins.
- `internal/viewport/wrap_test.go` — the fallback consumes a real
  wrap cell (`ab\x01◌́cd` at width 4 wraps to `ab^A` / `◌◌́cd`).
- `internal/app/cellmodel_test.go` —
  `TestComposedViewStandaloneMarkFallbackCell`: the composed terminal
  output paints the highlighted `◌◌́` cell, the following cluster in
  the next cell, and the same one-cell geometry under panning; the
  rewritten indicator/reveal/completion tests pin the shifted spans.

## Files

- `internal/present/line.go` — `emit`'s zero-width branch creates the
  fallback cell; the ASCII/cluster boundary keeps `lead` zero-width
  emits standalone-only.
- `Notes/decisions/043-combining-cluster-fallback-cell.md` — the
  recorded representation, byte sequence, and normalization contract.

See also: [safe-presentation.md](safe-presentation.md) (the shared
grapheme/cell policy), [grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)
(the `clusterSpan` expansion these cells feed),
[wrap-mode.md](wrap-mode.md) and
[horizontal-panning.md](horizontal-panning.md) (the cell consumers),
[zero-width-match-markers.md](zero-width-match-markers.md) (marker
positions on fallback bytes).
