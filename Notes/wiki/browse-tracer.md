# Browse tracer: file list, file panel, async loading (Issue #5)

Delivered by
[Issue #5](../issues/005-browse-tracer-file-list-and-file-panel.md):
the interim summary from [Issue #3](search-collection.md) is replaced by
the real two-pane browse view — a file list in raw-path order beside the
current file's content with matches in inverse video — plus the first
paths through FileBuffer, Viewport, and Theme, and the safe-presentation
core that every rendered sink routes through. Relevant PRD sections:
*File list and layout* (stories 24–26, 31, 34), *File loading, cache,
reload, and selection consistency* (first two bullets), *Layout and
indicators* (gutter bullet), *Text, graphemes, and safe presentation*,
and *Module Design → FileBuffer / Viewport / Theme / App* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md).

## Safe-presentation core (`internal/safepresentation`)

Issue #5 is the first to render arbitrary external bytes, so it lands the
escaping core ahead of the first render;
[Issue #6](safe-presentation.md) has since generalized it into the
shared all-sink utility and replaced the Issue #1 `cli.Escape` escaper.
The contract is narrow: raw control sequences from external
data never execute, and original bytes remain the key for identity,
ordering, and file access — displayed strings never become filesystem
keys.

- **Paths** — `EscapePath([]byte) string`: `\n`, `\r`, `\t` become the
  two-character escapes; a literal backslash doubles; other C0 controls
  and DEL use caret notation (ESC is `^[`, DEL is `^?`); C1 controls and
  non-printable runes use `\uXXXX` escapes; invalid UTF-8 bytes use
  `\xNN`; valid printable Unicode is preserved.
- **Content** — `MapContent(raw []byte) Mapped`: line-splitting owns
  terminators (LF and CRLF never reach this function); a standalone CR
  renders as `^M`; other C0 controls and DEL use caret notation; C1
  controls and non-printable single runes use `\uXXXX` escapes; invalid
  UTF-8 bytes each render as a U+FFFD cell (empty `Text`, retained
  byte mapping); a tab renders as a single provisional `→` cell —
  the structural eight-column-stop rule is Issue #16's, and no test
  asserts cell positions on tab-containing lines.
- **Byte→cell maps** — `Mapped{Text, Cells}` pairs the display text
  with one `Cell{Text, Start, End}` per terminal cell recording the
  half-open source-byte range that produced it. `CellsCovering(start,
  end)` maps a byte range to the cell range covering every cell those
  bytes produced, so a match covering an ESC byte highlights both `^`
  and `[`, and a match inside a C1 escape's bytes covers the whole
  `\uXXXX` form. Wide graphemes occupy multiple cells that all map to
  the cluster's bytes; a cluster with no visible cell gets the Issue #43
  `◌` fallback cell.
- **Cell policy** — `CellWidth` is the shared uniseg cell measurement;
  `decodeRune` is kept inside the package so display-geometry code
  elsewhere never decodes runes itself (the Issue #39 boundary).

## FileBuffer (`internal/filebuffer`)

`Load(path []byte, stops []searchindex.Stop) (*Buffer, error)` performs
the whole load — disk read plus decode/map — so a completion message
carries a prepared buffer and the update path does no full-file work.
The raw path bytes are the filesystem key. Line splitting counts LF and
CRLF as terminators (never displayed, retained in `Line.Raw`), a missing
final newline still yields the last line, a trailing newline does not
invent an empty line, and an empty file has zero lines. Each `Line`
embeds the safe-presentation `Mapped` (`Text` + `Cells`), keeps the
original raw bytes including the terminator, and carries `Highlights`:
each stop's recorded union byte ranges mapped through
`Mapped.CellsCovering` to display-cell ranges (spans wholly on removed
terminator bytes cover no cells yet — terminator markers are Issue
#22–23's). `GutterWidth` is the digit width of the largest line number
plus two spaces, minimum one digit slot. Stale-match validation is Issue
#29's; UTF-16/32 classification is Issue #30's.

## Viewport and Theme seams

`internal/viewport` lands only the seam's first responsibility: a
`Viewport` is a vertical window over a loaded buffer — `Visible(lines,
height)` returns `[top, top+height)` and the zero value shows the top of
the file. Scrolling, destination reveal, wrap, logical anchors, and
horizontal state arrive in Issues #12–21.

`internal/theme` lands the initial scheme `Dark()` (white on black;
`Inverse` for match runs, `Underline` for the current list entry) and
`Plain()`, the no-style composition path where every decorator is the
identity — the sink-safety tests' way to prove no escape byte may
legitimately appear. The `c` toggle and full style set are Issue #7's.

## App browse composition (`internal/app`)

`searchDoneMsg` now transitions to `stateBrowse` (the interim
`stateSummary` screen is gone), stores the prepared index, and returns
the current file's load command. The model keeps `loading` (in-flight),
`bufs` (prepared buffers), and `failed` (read failures), all keyed by
raw path bytes; `startLoad` drops repeated requests for a path already
loading rather than queueing them, and the load command runs the whole
read-plus-decode/map under the optional `loadGate` before returning
`fileLoadedMsg{path, buf, err}`. A completion for a non-current path
updates only its cache slot — it cannot replace the visible panel — and
the `quitting` discard drops completions that land after cancellation.
`q` in browse exits with the fixed status 0 through the Issue #4
`quitCmd` cleanup; `ctrl+c` still exits 130; resize and stray keys are
handled while a load is gate-held. `VRG_TEST_LOAD_GATE=<file>` in
`cmd/vrg` wires the seam (see
[cancellation-cleanup.md](cancellation-cleanup.md) for the seam family).

## Layout

The frame is exactly `height` rows:

- Row 0 is the **filename rule** across the full width: `─ <escaped
  current path> ────` (the path clips at the boundary).
- Below it, the **file list** occupies a fixed heuristic column — the
  longest escaped path width plus one cell, capped at the terminal
  width; the current entry is underlined and the list scrolls to keep
  it visible. Issue #24 owns the real formula (40% cap, minimum text
  width, `…` left-truncation) and the hide/show keys.
- The **file panel** shows a right-justified line-number gutter
  (`GutterWidth` digits + two spaces) followed by escaped content cells
  clipped to the text width without splitting a grapheme; matched cell
  ranges render in inverse video as maximal styled runs. An invalid
  byte's empty-`Text` cell emits U+FFFD so invalid content is visibly
  replaced. No left/right/bottom panel border is drawn.
- Until the current file's buffer arrives the panel shows "Loading…";
  a read failure shows "(unreadable)". The first visit starts at the
  top of the file — no reveal yet (Issue #14).

## Sink-safety method

`TestHostileFixtureRawOutput` drives the hostile fixture — a filename
containing a newline, an ESC byte, and an invalid UTF-8 byte, plus a
matched line carrying OSC, CSI, C0 controls, a C1 control, DEL, a
standalone CR, and an invalid byte — through the **real** composition
path (`searchDoneMsg` → load command → `fileLoadedMsg` → `View()`) on
the `theme.Plain()` no-style path. It asserts on the raw output, before
any ANSI stripping, that the view is valid UTF-8 carrying no control
rune except the row-separator newlines, that the frame is still exactly
`height` rows (a survived filename newline would add one), and that the
escaped forms appear in all three sinks — file-list entry, filename
rule, and panel content. [Issue #6](safe-presentation.md) has
restructured this fixture set into the shared, extensible sink-safety
table (`internal/safepresentation/sinktest` +
`internal/app/sinksafety_test.go`), which adds the usage-error-stderr
and CLI-help-stdout rows alongside the three browse sinks.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/safepresentation/safepresentation_test.go` covers the path and
content escape rules and the byte→cell maps;
`internal/filebuffer/filebuffer_test.go` covers loading, line counts,
gutter width, escaped content, and cell-mapped highlights;
`internal/app/browse_test.go` covers the browse transition, the
placeholder, gated-load responsiveness, the underline/gutter/rule
rendering, exit paths, late-load isolation, and the hostile-fixture
raw-output checks; `cmd/vrg/search_test.go` now proves record survival
at the PTY boundary through inverse-video span counts.
