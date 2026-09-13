## Issue 43: Standalone combining clusters get a real one-cell display fallback

**Type**: HITL — the PRD requires a visible fallback cell but does not select its glyph; the exact fallback representation is a human product decision (see below)
**Blocked by**: Issue 39 — the fallback cell must propagate through the cluster-based renderer that issue installs; landing it first avoids re-doing this propagation

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 8

### What to build

Materialize a real fallback cell for zero-width standalone combining clusters (`internal/filebuffer/filebuffer.go:203-218`, `382-399`). The current code assigns a synthetic width of one only to `clusterCells`, but leaves the cluster's own width and display text unchanged and advances `cellPos` by the original zero width. A following cluster can therefore overlap the supposed fallback, and the renderer has no actual independent cell to paint. Existing tests assert a non-zero highlight range but never prove a visible fallback cell exists.

**Decision required first (HITL)** — choose exactly one deterministic fallback representation and record it (in this issue and the PRD's *Text, graphemes, and safe presentation* section or a `Notes/decisions/` entry) before implementation begins:

- **Candidate A — dotted-circle base**: display `U+25CC ◌` followed by the original combining marks, so the marks compose onto a conventional "no base" carrier.
- **Candidate B — space base**: display a space followed by the original combining marks.
- **Candidate C — fixed placeholder**: display a single visible glyph (e.g. U+FFFD or U+25CC) *instead of* the original marks.

The decision must state: (a) the exact display byte sequence; (b) its expected grapheme segmentation and cell width under the shared `rivo/uniseg` policy — it must segment as a single cluster occupying exactly one terminal cell, and the decision must define how an unexpected width result (0 or >1 from the width library, or an inconsistent terminal) is normalized to the required one cell; (c) that byte-to-cell mapping still resolves the fallback cell to the cluster's original source bytes.

Once the representation is chosen:

- Give a standalone combining cluster the chosen one-cell display representation rather than only a widened cell annotation.
- Propagate the fallback width consistently through clusters, byte-to-cell mappings, wrapping, clipping, highlight expansion, and rendering — every layer agrees the cluster occupies one cell.
- Byte mapping still points at the original source bytes; only the *display* gains the fallback cell.

See PRD *Text, graphemes, and safe presentation* ("a cluster without a base/independent visible cell must receive a visible fallback cell") and Issue 39's cluster-based render path.

### How to verify

- **Manual**: view a file whose line begins with (or contains) a standalone combining mark with no base character → it occupies one visible cell showing the chosen fallback representation, the following character renders in the next cell with no overlap, and a match covering the mark highlights exactly that one cell.
- **Automated**: a composed terminal-output test asserting the fallback paints exactly one cell with the *chosen* display bytes/grapheme (not merely that the next cluster starts one cell later) followed correctly by the next cluster; unit tests asserting `cellPos` advances by the fallback width, byte mappings still resolve to the cluster's source bytes, and wrapping/clipping treat the fallback as a real cell.

### Acceptance criteria

- [ ] Given the fallback representation has been decided and recorded, then a standalone combining cluster occupies exactly one display cell showing that representation.
- [ ] Given a cluster following the fallback, then it renders in the next cell — no overlap and no shared cell.
- [ ] Given byte-coordinate mapping (matches, highlights), then it still resolves the fallback cluster to its original source bytes.
- [ ] Given wrapping, clipping, and horizontal panning, then the fallback cell is counted like any other cell by every layer.
- [ ] Given a match covering the standalone cluster, then the highlight paints the fallback cell and nothing adjacent.
- [ ] Given the tests, then they assert the recorded display bytes/grapheme and cell width, so a silently different fallback convention fails.

### User stories addressed

- User story 71: one consistent grapheme/cell policy for wrapping, clipping, and highlights
- User story 72: partial-grapheme matches highlight the whole grapheme
- User story 75: invalid content and controls escaped/visible in every output sink

---
