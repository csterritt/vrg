## Issue 43: Standalone combining clusters get a real one-cell display fallback

**Type**: AFK
**Blocked by**: Issue 39 — the fallback cell must propagate through the cluster-based renderer that issue installs; landing it first avoids re-doing this propagation

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Medium finding 8

### What to build

Materialize a real fallback cell for zero-width standalone combining clusters (`internal/filebuffer/filebuffer.go:203-218`, `382-399`). The current code assigns a synthetic width of one only to `clusterCells`, but leaves the cluster's own width and display text unchanged and advances `cellPos` by the original zero width. A following cluster can therefore overlap the supposed fallback, and the renderer has no actual independent cell to paint. Existing tests assert a non-zero highlight range but never prove a visible fallback cell exists.

- Give a standalone combining cluster a concrete one-cell display representation (e.g. a visible escaped/placeholder form on a space base, matching the grapheme policy's fallback convention) rather than only a widened cell annotation.
- Propagate the fallback width consistently through clusters, byte-to-cell mappings, wrapping, clipping, highlight expansion, and rendering — every layer agrees the cluster occupies one cell.
- Byte mapping still points at the original source bytes; only the *display* gains the fallback cell.

See PRD *Text, graphemes, and safe presentation* (single consistent grapheme/cell policy) and Issue 39's cluster-based render path.

### How to verify

- **Manual**: view a file whose line begins with (or contains) a standalone combining mark with no base character → it occupies one visible cell as an escaped placeholder, the following character renders in the next cell with no overlap, and a match covering the mark highlights exactly that one cell.
- **Automated**: a composed terminal-output test asserting the fallback occupies exactly one painted cell followed correctly by the next cluster; unit tests asserting `cellPos` advances by the fallback width, byte mappings still resolve to the cluster's source bytes, and wrapping/clipping treat the fallback as a real cell.

### Acceptance criteria

- [ ] Given a standalone combining cluster, then it occupies exactly one display cell with a visible fallback representation.
- [ ] Given a cluster following the fallback, then it renders in the next cell — no overlap and no shared cell.
- [ ] Given byte-coordinate mapping (matches, highlights), then it still resolves the fallback cluster to its original source bytes.
- [ ] Given wrapping, clipping, and horizontal panning, then the fallback cell is counted like any other cell by every layer.
- [ ] Given a match covering the standalone cluster, then the highlight paints the fallback cell and nothing adjacent.

### User stories addressed

- User story 71: one consistent grapheme/cell policy for wrapping, clipping, and highlights
- User story 72: partial-grapheme matches highlight the whole grapheme
- User story 75: invalid content and controls escaped/visible in every output sink

---
