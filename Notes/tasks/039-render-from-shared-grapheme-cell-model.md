# Tasks for #39: Final rendering uses the shared grapheme/cell model end to end

Parent issue: #39
Parent PRD: PRD-vrg.md
**Blocked by issues**: #38
**Acceptance criteria**: AC1–AC6 → Tasks 1–2
**Manual verification**: Task 3 owns the issue's manual checks.

## Tasks

### 1. Specify cluster-based rendering and the shared width helper

**Type**: RED  
**Output**: Failing composed-view tests assert cluster-exact highlights, combining and ZWJ handling, overlay/list/filename/pop-up sizing for wide text, and a mechanical guard rejecting rune-decoding width or truncation loops outside the shared grapheme/cell helper.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #38 is complete — the renderer must draw into the correct content-panel width before its cell-level output can be verified against it. Add failing tests in `internal/app` (composed `View()` render assertions inspecting the emitted cell layout), `internal/theme`, and the relevant packages covering: a match overlapping a two-cell CJK character highlights exactly that cluster's cells and never swallows the following character; a base-plus-combining sequence is styled, clipped, and truncated only as one cluster; an emoji ZWJ sequence occupies its measured cell width and is never split by a highlight or clip boundary; Theme overlay sizing and padding use measured cell widths for wide or combining text so borders align; file-list entry padding, indicator-column sizing, filename-row fitting, and file-change pop-up truncation/width/centering use grapheme boundaries for wide and combining paths; and the pop-up no longer assumes `safepresentation.EscapePath` produces no combining marks. Add a source-scanning static guard with an exact file allow-list: in non-test production Go files, the symbol `utf8.DecodeRuneInString` may appear only in `internal/safepresentation/cellwidth.go`, the shared grapheme/cell-helper implementation. The guard scans every non-test production `.go` file under `internal/` and `cmd/`; any occurrence outside that one allow-listed file fails, including an occurrence in a newly added package or file, so display-geometry code in `internal/viewport`, `internal/filebuffer`, or any later consumer cannot evade the shared policy. Existing non-geometry byte decoding in the scanned production packages must be rewritten with `for range`, an existing standard-library primitive, or another implementation that does not call the allow-listed symbol; test-only decoder utilities are excluded from the scan. Keep this task test-only.

---

### 2. Route all display geometry through the shared grapheme/cell helper

**Type**: GREEN  
**Output**: The composed-view tests pass; `renderLineWithHighlights`, `cellToBytePos`, `visibleWidth`, list-entry padding, indicator sizing, filename-row fitting, pop-up truncation/centering, and Theme overlay sizing all use one ANSI-aware grapheme/cell helper, and the file panel renders from `Line.Clusters`.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Introduce one shared ANSI-aware cell-width helper implementing the existing `rivo/uniseg` grapheme policy — place it in `internal/safepresentation/cellwidth.go` beside `GraphemeClusters` — and route every display-width and truncation consumer through it. That dedicated file is the sole production-file allow-list entry for `utf8.DecodeRuneInString`; remove every other occurrence from non-test production files under `internal/` and `cmd/`, and implement unrelated decoding there without that symbol so the static predicate remains mechanically decidable across every display-geometry package. Render the file panel directly from `Line.Clusters` and their cell spans in `renderLineWithHighlights` rather than re-deriving positions from runes or bytes; rewrite `cellToBytePos` and `visibleWidth` on cluster boundaries; apply the helper to file-list entry padding in `renderBrowse`, indicator-column sizing, filename-row fitting, and `theme.Overlay`/`cellWidth`. Replace `truncateLeftCells`'s rune-boundary implementation and its false no-combining-marks assumption, routing the pop-up through `TruncateLeftGrapheme` or the same shared cluster/cell primitive so printable wide and combining path text is never split or miscentered. Ensure highlight and truncation boundaries fall only on grapheme-cluster boundaries measured in cells. Leave the shared helper and width-independent escaped-path/cluster geometry as the explicit handoff consumed by Issue #40; Issue #40 must preserve this renderer and rerun these focused regressions while moving grouping work out of `View()`. Run the focused tests plus `go build ./...`, `go vet ./...`, and `go test ./...`.

---

### 3. Create the unified-rendering walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/039-04/code-walkthrough`.  
**Depends on**: 2

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/039-04/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate the composed-view tests and the static guard, then run the issue's manual scenario: browse a file containing CJK text, a combining sequence such as `e` plus combining acute, and an emoji ZWJ sequence with a match overlapping them → each highlight covers the whole cluster at exactly its cell width, overlay text containing wide characters is sized and padded correctly, and nothing is clipped mid-cluster. Capture commands, outputs, and exit statuses. Reference Issue #39 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
