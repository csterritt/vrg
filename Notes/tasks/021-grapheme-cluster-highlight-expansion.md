# Tasks for #21: Grapheme-cluster highlight expansion and wide-glyph safety

Parent issue: #21
Parent PRD: PRD-vrg.md
**Blocked by issues**: #19, #20
**Acceptance criteria**: AC1–AC4 → Tasks 1–2
**Manual verification**: Task 4 owns the issue's manual checks.

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

### 3. Document grapheme highlight expansion

**Type**: DOCUMENT  
**Output**: Wiki documentation records the expansion rules, fallback cells, and the expanded-span contract for reveal and indicators.  
**Depends on**: 2

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #21 implementation and tests into the appropriate pages under `Notes/wiki`. Document outward cluster expansion, combining-only matches, standalone-cluster fallback cells, wide glyphs never split with blanks never painted as match cells, and the contract that the expanded span is the single source for highlights, reveal, and indicator visibility. Cross-reference Issue #21 and the Text, graphemes, and safe presentation section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 4. Create the expansion walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/021-04/code-walkthrough`.  
**Depends on**: 3

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/021-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the expansion, fallback, and wrap-boundary rendering tests and the updated indicator tests, then run the manual cases: searching a file containing decomposed `e\u0301` for the combining mark bytes themselves (not precomposed `é`, which ripgrep will not match) so the whole `é` glyph is highlighted, a CJK search showing both cells highlighted, and a standalone combining mark at line start producing a visible highlighted fallback cell. Reference Issue #21 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
