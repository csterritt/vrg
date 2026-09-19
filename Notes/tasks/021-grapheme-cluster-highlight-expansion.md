# Tasks for #21: Grapheme-cluster highlight expansion and wide-glyph safety

Parent issue: #21
Parent PRD: PRD-vrg.md
**Blocked by issues**: #19, #20
**Acceptance criteria**: AC1–AC4 → Tasks 1–2

## Tasks

### 1. Specify grapheme-cluster highlight expansion

**Type**: RED  
**Output**: Failing FileBuffer tests cover partial-cluster expansion in all positions, combining-only matches, standalone-cluster fallback cells, wide pairs, and ZWJ sequences; failing rendering tests cover wrap-boundary blanks; failing indicator tests consume the expanded spans.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #19 and #20 are complete. Add failing tests in `internal/filebuffer` and the rendering path for the Issue #21 contracts and the grapheme expansion bullets of the Text, graphemes, and safe presentation section of `Notes/PRD-vrg.md`. Require a nonempty span partially covering a cluster to expand outward to the entire cluster (start inside, end inside, and both); a combining-only match within a base cluster to highlight that whole cluster; a cluster with no base or independent visible cell to receive a visible fallback cell so the highlight is never zero cells; wide glyphs never split by highlight boundaries, with wrap and clip blanks from Issues #16 and #18 never painted as match cells, including a highlight at a wrap boundary not painting the blank filler cell; an emoji ZWJ sequence handled by the shared policy; and the cluster-expanded span to be what FileBuffer hands to Viewport and App so reveal and indicators consume it — update the Issue #20 indicator tests so a match starting mid-cluster whose cluster start is hidden left counts as hidden left. Keep this task test-only.

---

### 2. Implement cluster-expanded highlight spans

**Type**: GREEN  
**Output**: Expansion, fallback, wide-glyph, rendering, and updated indicator tests pass with expanded spans feeding reveal and indicators.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement cluster expansion in `internal/filebuffer` to satisfy Task 1: outward expansion of partial spans, combining-only matches highlighting their base cluster, the visible fallback cell for clusters without one, and wide-cluster cell pairs — then hand the expanded spans to Viewport and App as the sole span source so the Issue #14 and #19 reveals and the Issue #20 indicators consume them unchanged.

---

### 3. Create the expansion finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/021-04/finish-marker.md`.  
**Depends on**: 2

Write `Task 021-04 finished successfully at <time>` to `Notes/finish-markers/021-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/021-04/` directory if it does not already exist.

---
