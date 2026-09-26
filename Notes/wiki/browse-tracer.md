# Browse tracer — file list and file panel

Issue #5 (`Notes/issues/005-browse-tracer-file-list-and-file-panel.md`,
tasks `Notes/tasks/005-browse-tracer-file-list-and-file-panel.md`)
replaced the interim `N files, M matched lines` summary with the first
arbitrary-data rendering path: the two-pane browse view. This is the
first screen to put attacker-controlled bytes — filenames and file
content — on the terminal, so it lands together with the safe-
presentation core it routes through.

PRD cross-references: "File list and layout", "Text, graphemes, and
safe presentation", and "Module Design" in `Notes/PRD-vrg.md`.

## Two-pane browse layout

`Model.renderBrowse` (`internal/app/browse.go`) composes each frame:

- **Left pane — file list.** Distinct raw paths in index order
  (unsigned raw path bytes, inherited from the Issue #3 stop ordering).
  The current entry is underlined via the theme, and the list window
  scrolls just enough to keep it visible. Width is provisional: the
  longest escaped path plus padding, capped at 40% of the terminal and
  by the file panel's minimum — Issue #24 owns the real formula.
- **Right pane — file panel.** Row 0 is the *filename rule*: `──`, the
  escaped current path, then dashes to the panel edge; an oversized
  path is left-truncated with a leading `…` so the basename tail stays
  visible. Rows below carry the current file's content. There are **no
  other panel borders** — no box drawing, no separator column between
  the panes beyond the list's own width padding.
- **Gutter.** Each content row starts with the right-justified line
  number followed by two spaces. Digit width is the decimal width of
  the loaded file's largest line number, minimum one slot
  (`Buffer.GutterWidth`/`GutterDigits`).
- **Matches.** Validated highlight spans render in inverse video over
  the escaped display text, driven by the byte→cell maps (below) so a
  match covering an ESC byte highlights both cells of its `^[` form.

## Async loading and prepared buffers

Full-file work never lands on the UI update path:

- `ensureLoad` starts one worker `tea.Cmd` per file (repeat requests
  are dropped, not queued). The worker runs `filebuffer.Load` — read,
  split, escape, map — and returns a `loadDoneMsg` carrying the
  finished `*filebuffer.Buffer` keyed by the raw path bytes.
- `Update` only installs the completed buffer (or marks the path
  failed); it performs no read/decode/map work itself.
- Until the buffer arrives the panel shows `Loading…` behind a minimal
  one-digit gutter; a failed load shows `(unreadable)`.
- `Model.loadGate` is the test seam: when set, the worker blocks on the
  channel before its read and decode/map phases, letting tests prove
  keys and resizes are still processed mid-load.
- The current file opens at top-of-file with no destination reveal
  (that is Issue #12's); the `viewport.Viewport` seam only carries
  content dimensions and the clamped visible range.

## Safe-presentation core

Issue #5 landed the escaping core here; Issue #6 generalized it into
`internal/present`, the shared all-sink utility — see
[safe-presentation.md](safe-presentation.md) for the canonical
contracts. In brief:

**Paths** (`present.Path`): `\n`, `\r`, `\t` become the two-character
forms `\n`, `\r`, `\t`; a literal backslash doubles to `\\`; invalid
UTF-8 bytes become `\xNN`; other C0 controls take caret notation
(`^[` for ESC) and DEL takes `^?`; C1 controls take `\uXXXX`; valid
printable Unicode passes through. The raw path bytes — never the
display form — remain the key for identity, ordering, and file access.

**Content** (`present.LineOf`): invalid UTF-8 renders as U+FFFD while
its raw-byte mapping is retained; C0 controls and DEL use caret
notation; C1 controls use `\uXXXX`-style escapes; LF and CRLF are
structural line terminators and are never displayed (their bytes map to
the end-of-line position); a standalone CR renders as `^M`; a tab
renders as the provisional single-cell `→` placeholder pending Issue
#16's stop expansion — no test may assert specific cell positions on
tab-containing lines. Grapheme clusters go through `x/ansi` width
accounting: a printable cluster is one unit; a cluster mixing
printable and dangerous forms falls back to per-rune rules so no
control byte survives verbatim.

**Byte→cell maps.** Every emitted unit records, for each source byte,
its first display cell (`lo`) and the cell after its last (`hi`).
`Line.Span` maps a raw byte range to its display span: interior
bytes expand to their whole unit (a rune's bytes share its cells; an
escape's source byte covers every cell of the escape). A range covering
no display cells — a match solely on removed terminator bytes, or a
zero-width position — yields a `Span` marker with `Start == End`,
rendered as one marked cell without shifting text.

## Sink-safety method

The three browse sinks — file list, filename rule, panel content — are
all rendered through `internal/present`. Issue #6 restructured the
hostile-fixture test into `TestSinkSafetyTable`
(`internal/app/sinksafety_test.go`): a shared fixture set (OSC
`\x1b]0;x\x07`, CSI `\x1b[2J`, C0 controls, a C1 control, DEL, a
standalone CR, invalid UTF-8 path bytes, an embedded filename newline)
driven through the real composition path of every sink — the three
browse sinks plus usage-error stderr and CLI-help stdout — with
`theme.Plain`, the no-style seam in which every style is the identity,
so rendered output may legitimately contain no escape bytes at all.
Assertions inspect the raw `View()` string before any ANSI stripping:
no fixture control byte survives verbatim, the escaped forms are
visible in each sink, and a styled pass proves no fixture payload
follows an unescaped ESC.

## Files

- `internal/present/present.go`, `line.go` — `Path`, `Diagnostic`,
  `LineOf`, `Line` (raw bytes + text + cells + `lo`/`hi` maps),
  `Cell`, `Span` — the shared utility moved here by Issue #6.
- `internal/filebuffer/filebuffer.go` — `Buffer`, `Load` (submatch
  validation against raw line bytes: out-of-bounds and mismatched
  ranges dropped), `LineCount`, `GutterWidth`/`GutterDigits`, `Text`,
  `Cells`, `Spans`.
- `internal/viewport/viewport.go` — minimal seam: dimensions plus
  `Range` clamped to the loaded line count.
- `internal/theme/theme.go` — `Dark` (real SGR) and `Plain` (identity
  styles) implementing `Inverse`/`Underline`.
- `internal/app/browse.go` — `loadDoneMsg`, `ensureLoad`/`loadCmd`,
  `renderBrowse`, `filenameRule`, `contentRow`, `renderCells`.
- `internal/app/app.go` — `phaseBrowse`, buffer/loading/failed maps,
  cursor/files/stops, `relayout`.

See also: [safe-presentation.md](safe-presentation.md) (the generalized
utility and sink-safety table),
[searchindex-records-and-stops.md](searchindex-records-and-stops.md)
(raw path identity and ordering the list inherits),
[cancellation-and-cleanup.md](cancellation-and-cleanup.md) (the exit
path `q`/`ctrl+c` still run through),
[search-spawn-and-searching-screen.md](search-spawn-and-searching-screen.md)
(the `Searching…` phase this view replaces the summary of).
