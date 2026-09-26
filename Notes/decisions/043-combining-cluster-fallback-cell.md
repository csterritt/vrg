# Decision record: Issue #43 standalone combining-cluster fallback cell

**Date**: 2026-09-25
**Status**: Recorded decision (Task 1 gate, Issue #43)
**References**: Issue #43's task spec `Notes/tasks/043-combining-cluster-fallback-cell.md` (issues 036+ have no standalone issue files; the tasks file is the issue's specification), `Notes/PRD-vrg.md` — *Text, graphemes, and safe presentation*.

## Decision

**Option A — dotted-circle base.** A standalone combining cluster — a
grapheme cluster that `rivo/uniseg` reports as its own zero-width unit
because it has no base — is displayed as **U+25CC DOTTED CIRCLE followed
by the cluster's original bytes, verbatim**.

The rejected alternatives:

- **Option B (space base)** renders identically-shaped cells but paints
  a bare space as the carrier: the fallback is visually
  indistinguishable from an ordinary space-plus-marks cluster, hiding
  the fact that the base was synthesized.
- **Option C (fixed glyph instead of the marks)** discards the original
  combining marks entirely, so distinct standalone clusters become
  indistinguishable and the display no longer carries the source's
  marks at all.

Option A keeps the source's marks composing onto U+25CC — the
conventional "no base" carrier — so the cell is visibly a fallback
while still showing which marks the source contained. It is also the
convention `present.lineOf` already used for the line-start case, so
the decision generalizes an established representation rather than
introducing a second one.

## (a) Exact display byte sequence

The fallback cell's text is the UTF-8 encoding of U+25CC — `EF 97 8C`
— followed by the standalone cluster's original source bytes
unchanged. A line containing only `U+0301` at its start displays the
byte sequence `EF 97 8C CC 81` ("◌◌́"); a standalone `U+0301 U+0302`
cluster displays `EF 97 8C CC 81 CC 82`.

Only *standalone* zero-width clusters take the fallback. Marks that
`rivo/uniseg` groups into a preceding cluster keep that cluster's base:
a mark following a printable base is already inside its cluster, and a
mark the policy attaches to an invalid byte's cluster composes onto the
U+FFFD replacement glyph that byte paints as. Zero-width *runes* inside
a multi-rune cluster (the per-rune escape path for clusters mixing
printable and dangerous forms) likewise keep composing onto their
cluster's preceding cell rather than taking a fallback of their own.

## (b) Segmentation and width expectation, with normalization

Under the shared `rivo/uniseg` policy (`ansi.FirstGraphemeCluster` /
`ansi.StringWidth`), `U+25CC` followed by the cluster's marks segments
as exactly **one grapheme cluster** — U+25CC is an ordinary base
followed by Extend runes (GB9 applies) — measuring exactly **one
terminal cell** (`StringWidth("◌́")` = 1, including multi-mark tails).

Normalization rule: the one-cell occupancy is **structural, not
measured**. The fallback emits exactly one `present.Cell` — never a
lead-plus-continuation pair, never a count derived from a width call —
so a width library returning 0 or >1 for the sequence cannot change the
cell model's accounting, and `Line.Width`, spans, wrapping, clipping,
and horizontal extents all count the fallback as one cell regardless.
A terminal that renders the sequence wider or narrower is out of scope
for the model: the byte→cell contract (below) is what the renderer
guarantees internally.

## (c) Byte-mapping contract

The fallback's added U+25CC is a display-only prefix: it is not a
source byte and occupies no byte→cell mapping slot. The cluster's
original source bytes map to the fallback cell through the line's
`lo`/`hi` tables exactly as before, so:

- `Line.Span(start, end)` over the cluster's bytes returns the fallback
  cell's one-cell range;
- a coverage match on those bytes highlights exactly the fallback cell
  and nothing adjacent (cluster-boundary expansion has nothing to grow
  into — the cell is its own `Lead`);
- a zero-width position inside the cluster maps to the fallback cell's
  index, the cluster's start cell.

## Where it applies

The rule lives in `internal/present`'s line escaping (`lineOf`/`emit`):
any zero-width unit that begins a new grapheme cluster — at line start
or after any break-eligible predecessor (control caret escapes, a tab
expansion, a standalone CR escape, or another standalone zero-width
cluster) — takes the fallback cell. `internal/filebuffer` surfaces it
through `Line.Cells`/`Line.Span`; the cluster-driven renderer, row
wrapping, clipping, horizontal panning, and highlight expansion consume
it like any other painted cell with no special-casing downstream.
