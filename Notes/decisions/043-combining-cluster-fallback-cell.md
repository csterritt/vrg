# Decision record: Issue #43 standalone-combining-cluster fallback representation

**Date**: 2026-09-15
**Status**: Decided at the Task 1 REVIEW gate during implementation (Issue #43 is marked HITL; the reviewing human declined to select a candidate at prompt time, so this records the implementer's selection with rationale — a different convention may be substituted by updating this record, the pinned display bytes, and the tests that assert them).
**References**: `Notes/issues/043-combining-cluster-fallback-cell.md`, `Notes/tasks/043-combining-cluster-fallback-cell.md`, `Notes/PRD-vrg.md` — *Text, graphemes, and safe presentation*.

## Selected representation: Candidate A — dotted-circle base

A standalone combining cluster (a grapheme cluster with no base and no
independent visible cell, i.e. `Cluster.Width == 0`) displays as
**`U+25CC ◌` DOTTED CIRCLE followed by the cluster's original combining
mark bytes**, so the marks compose onto the conventional "no base"
carrier and remain visually identifiable.

### (a) Exact display byte sequence

For a zero-width cluster occupying display bytes `[s, e)` of the escaped
line text, the display text gains `U+25CC` (UTF-8 `E2 97 8C`) inserted
immediately before byte `s`; the fallback unit shown in the cell is
`◌` + the cluster's original bytes. Example: a line whose first content
is `U+0301` (combining acute, `CC 81`) followed by `x` displays as
`E2 97 8C CC 81 78` — `◌́x`. The dotted circle is display-only: no
source byte produces it.

### (b) Segmentation, width, and normalization

Under the shared `rivo/uniseg` policy, `◌` followed by combining marks
segments as a single grapheme cluster (UAX #29 GB9: no break before
extending characters) and `uniseg.StringWidth` reports width 1 — the
expected result is exactly one cluster occupying exactly one terminal
cell.

Normalization rule for unexpected results: the fallback cell is
*constructed* as one cell rather than re-measured. The cluster table
records the fallback unit (`◌` + the original cluster's bytes) as a
single cluster with `Width` pinned to 1, so a width-library report of 0
or >1 for a pathological mark sequence, or a terminal that renders the
composed unit inconsistently, cannot change the recorded cell geometry.
Byte-to-cell mappings, wrapping, clipping, highlight expansion, panning,
and the Issue #39 renderer all consume that recorded one-cell width.

### (c) Byte-to-cell mapping contract

Byte-to-cell mapping still resolves the fallback cell to the cluster's
original source bytes: every source byte of the former zero-width
cluster maps to the fallback cell's `[cell, cell+1)` range. The
inserted `U+25CC` bytes carry no source-byte mapping; display byte
offsets of subsequent source bytes shift by the three inserted bytes.
Only the *display* gains the fallback cell — match/highlight mapping,
stale validation, and raw-byte coordinates are unaffected.

## Rejected candidates

- **Candidate B — space base**: marks compose onto an invisible carrier;
  the mark identity is harder to see and terminal rendering of
  space+marks is less consistent than the purpose-built dotted circle.
- **Candidate C — fixed placeholder**: discards the marks' visual
  identity entirely and, for `U+FFFD`, conflates combining marks with
  the invalid-UTF-8 replacement glyph already used by `EscapeContent`.
