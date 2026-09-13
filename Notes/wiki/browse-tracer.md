# Browse tracer (Issue #5)

The two-pane browse view delivered by
[Issue #5](../issues/005-browse-tracer-file-list-and-file-panel.md):
after a completed search with results, the app enters a browse state
showing an ordered file list on the left and the current file's content
on the right with matches highlighted in inverse video. Relevant PRD
sections: *File list and layout*, *Text, graphemes, and safe
presentation*, *Module Design → FileBuffer / Viewport / Theme / App*,
*Navigation, viewport, and logical anchors*, and *Outcome and
exit-status contract*.

## Safe-presentation core

`internal/safepresentation` is the shared safe-presentation utility
for every output sink. Issue #5 landed the path and content rules;
Issue #6 added the diagnostic escaper and unified the Issue #1
`cli.Escape` escaper onto `EscapePath`. See
[safe-presentation](safe-presentation.md) for the full contracts.

### Path escaping

`EscapePath(raw []byte) PathDisplay` escapes raw path bytes for safe
single-line display:

- `\n`, `\r`, `\t` → `\n`, `\r`, `\t` (backslash escapes)
- literal `\` → `\\`
- invalid UTF-8 bytes → `\xNN`
- other C0 controls → caret notation (`^X`)
- DEL (0x7f) → `^?`
- C1 controls (U+0080–U+009F) → `\u00XX`
- valid printable Unicode → preserved

`PathDisplay.ByteCells[i]` records the `[start, end)` display cell
range occupied by original byte `i`, so highlight rendering can map
byte ranges to cells. Displayed strings never become filesystem keys;
callers retain original bytes for identity, ordering, and file access.

### Content escaping

`EscapeContent(raw []byte) ContentDisplay` escapes raw content bytes
for safe display:

- invalid UTF-8 → U+FFFD (one cell), raw-byte mapping retained
- C0 controls and DEL → caret notation (`^[` for ESC, `^?` for DEL)
- C1 controls → `\u00XX`-style escapes
- LF and CRLF → line terminators, never displayed; bytes map to
  end-of-line position
- standalone CR (not followed by LF) → `^M`
- tab → single `→` placeholder cell (provisional pending Issue #16's
  eight-column-stop expansion)
- valid printable Unicode → preserved

`ContentDisplay.ByteCells[i]` records the `[start, end)` display cell
range for each original byte. A match covering an ESC byte highlights
both cells of `^[`.

## FileBuffer

`internal/filebuffer/filebuffer.go` loads, decodes, and maps a file's
bytes into a display-ready `Buffer`:

- `Buffer` — `Lines []Line`, `LineCount int`, `GutterWidth int`.
- `Line` — `Number` (1-based source line), `Display` (escaped text),
  `ByteCells` (per-byte cell ranges), `Highlights` (display cell ranges
  for inverse video).
- `Load(path, stops)` — reads the file at the raw path, splits it into
  lines (LF and CRLF are terminators; standalone CR is content), escapes
  each line through `safepresentation.EscapeContent`, and maps
  `Stop.Submatches` to display cell ranges via the byte→cell map.
- `gutterWidth(lineCount)` — digit count of the largest line number
  plus two spaces, minimum one digit.

The completion message carries a fully prepared buffer so `Update` does
no full-file work. Line splitting recognizes LF and CRLF as terminators;
a trailing terminator does not produce an extra empty line. A
standalone CR is content, not a terminator.

## Viewport

`internal/viewport/viewport.go` is the scrollable content view. Issue
#5 landed a minimal seam (`Lines`, `Height`, `Offset` always 0). Issue
#12 expanded it into the full manual vertical scrolling and
prepared-row rendering module. See
[manual-vertical-scrolling](manual-vertical-scrolling.md) for the
complete scroll-unit, clamping, per-file state, and render-cost
contracts.

## Theme

`internal/theme/theme.go` owns the active colour scheme and styles.
Issue #7 expanded the Issue #5 minimal seam into the full PRD style
set. See [theme-module](theme-module.md) for the complete contracts.

- `New()` — default theme with the dark scheme active (white on
  black) and styling enabled.
- `NoStyle()` — disables all ANSI sequences for sink-safety testing.
- `Scheme()` — reports the active scheme (`SchemeDark` or
  `SchemeLight`).
- `Toggle()` — flips the scheme between dark and light with no
  persistence.
- `Base(s)` — wraps s in the base colour pair and resets.
- `Match(s)` — wraps s in the true-inverse match colours and
  restores base.
- `CurrentMatch(s)` — true-inverse match colours + underline,
  restores base.
- `Indicator(s)` — inverse indicator style (same colours as Match),
  restores base.
- `Underline(s)` — SGR 4 underline, restores base.
- `Gutter(s)`, `FileList(s)` — base colours, reset.
- `FilenameRule(name)` — base-coloured horizontal rule with the name.
- `Overlay(s)` — base colours with a plain single-line border.

The no-style theme produces no ANSI escape sequences, so any control
byte in the output must come from unsanitized external data. This is
the sink-safety testing path.

## App browse composition

`internal/app/app.go` extends the Issue #3/#4 model with the browse
state:

### New state and messages

- `StateBrowse` — the two-pane browse state, entered after a completed
  search with results.
- `SearchCompleteMsg` — now carries `Index *searchindex.Index` in
  addition to `Files` and `Lines`. A non-nil index with files > 0
  transitions to `StateBrowse`; otherwise the backward-compatible
  `StateSummary` path is used.
- `FileLoadCompleteMsg` — carries the fully prepared `*filebuffer.Buffer`
  and the raw path. `Update` stores the buffer and clears the loading
  flag. Late completions after cancellation are ignored.

### New options

- `WithTheme(theme.Theme)` — injects the visual theme.
- `WithFileLoader(FileLoader)` — injects a file-loading function (test
  seam).
- `WithFileLoadGate(chan struct{})` — holds file loading until the
  channel is closed or receives (test seam for responsiveness).

`FileLoader` is `func(path []byte, stops []searchindex.Stop)
(*filebuffer.Buffer, error)`. The default loader is
`filebuffer.Load`.

### Browse model fields

- `index *searchindex.Index` — the prepared navigation index.
- `cursor *searchindex.Cursor` — the circular matched-line cursor
  (Issue #13). The single global navigation anchor; current file and
  current matched line derive from it. Startup selects the first stop.
- `buffer *filebuffer.Buffer` — the loaded content for the current
  file.
- `loading bool` — true while the file load is pending.
- `theme theme.Theme` — the visual theme.
- `fileLoader FileLoader` — the injected or default loader.
- `fileGate chan struct{}` — the file-load gate.
- `loadCancel chan struct{}` — cancellation channel for the file load.
- `viewport *viewport.Viewport` — the scrollable content view (Issue
  #12). `nil` while loading or no buffer.
- `currentPath []byte` — raw path of the currently loaded file (Issue
  #12).
- `perFileOffset map[string]int` — per-file saved vertical offset
  keyed by raw path (Issue #12).
- `rowProviderFactory RowProviderFactory` — test seam for the
  render-cost guard (Issue #12).
- `fileCache map[string]*filebuffer.Buffer` — session cache of loaded
  buffers keyed by raw path (Issue #13). No eviction; the PRD retains
  successful buffers for the session, so a revisited file can be shown
  immediately without a reload.

### Update flow

On `SearchCompleteMsg` with a non-nil index and files > 0:

1. Transitions to `StateBrowse`, stores the index, creates the cursor
   (`searchindex.NewCursor(m.index)`, which selects the first stop),
   sets `loading = true` (Issue #13: replaced the Issue #5
   `browseIdx = 0` with cursor creation).
2. Returns `m.loadFile()` — a `tea.Cmd` that asynchronously loads the
   cursor's current file.

`loadFile` (Issue #13: now delegates to `loadFileFor` with the cursor's
current stop's raw path):

1. Gets the current file's raw path from the cursor's current stop.
2. Waits at the file gate if set (cancellable via `loadCancel`).
3. Calls the file loader (injected or `filebuffer.Load`).
4. Returns a `FileLoadCompleteMsg` with the prepared buffer.

`loadFileFor(path []byte)` (Issue #13) is the path-keyed load command
used for cross-file navigation to an uncached destination. It collects
the stops for the given raw path, waits at the gate, calls the loader,
and returns a `FileLoadCompleteMsg`.

On `FileLoadCompleteMsg`:

1. If cancelled, ignores the message (late-load rejection).
2. Stores the buffer, caches it in `fileCache` keyed by raw path
   (Issue #13), and clears `loading`.
3. Issue #12: builds the viewport from prepared row data (via the row
   provider factory or `viewport.BufferRows`), restores the saved
   per-file offset for the path (0 for a first visit), and creates the
   viewport with the panel height and saved offset.

Key handling in browse state:

- `q` — exits with code 0 through the Issue #4 cleanup path (cancels
  process and load, quits).
- `ctrl+c` — exits with code 130 through the Issue #4 cancellation path.
- `c` — toggles the theme between dark and light (Issue #7), no
  persistence.
- Issue #13 navigation keys (active whenever `StateBrowse` is active,
  including while loading):
  - `n` — advances the matched-line cursor to the next stop
    circularly.
  - `p` — retreats the matched-line cursor to the previous stop
    circularly.
  - With zero or one stop, both are strict no-ops: no state change, no
    load command, no pop-up.
  - On a same-file move, only the current matched line styling
    changes; destination reveal belongs to Issue #14.
  - On a cross-file move, the departing file's viewport offset is
    saved, the content panel switches immediately, and a load is
    requested for an uncached destination; a cached destination is
    shown immediately with its saved viewport restored (first visit
    starts at the top).
  - Manual scrolling does not move the cursor, so `n`/`p` continue
    from the last selected stop, not the manually visible line.
- Issue #12 scroll keys (active only when the viewport is non-nil):
  - `up` / `down` — scroll one rendered row.
  - `u` / `d` — scroll half a page (`max(1, floor(contentHeight/2))`).
  - `pgup` / `pgdn` — scroll a full page (`contentHeight`).
  - After each scroll, the offset is saved as per-file state.
  - While the viewport is `nil` (loading placeholder), scroll keys are
    no-ops.
- Other keys and resize — handled without blocking, even while a load
  is pending. `WindowSizeMsg` calls `viewport.SetPanelHeight` to
  recompute layout and clamp the offset (Issue #12).

### Rendering

`View` renders `StateBrowse` via `renderBrowse`:

- **Base colours** — each composed line is wrapped in `theme.Base()`
  for the active scheme's base colour pair (Issue #7).
- **File list (left pane)** — each file's raw path escaped through
  `safepresentation.EscapePath`, in raw-path order (the index's
  unsigned byte ordering). The current file is underlined via
  `theme.Underline`. Issue #13: the current file derives from the
  cursor's current stop's raw path, so the underline follows cursor
  selection. A simple fixed/heuristic width is used (Issue #24 owns
  the real formula).
- **Content panel (right pane)** — the escaped filename embedded in a
  horizontal rule (`── name ──`), followed by content rows or
  `Loading…` while the buffer is unavailable. Issue #12: when the
  viewport is active, `renderContentPanel` queries `viewport.Visible()`
  for the visible row range only instead of scanning the full buffer
  per frame.
- **Gutter** — right-justified line number padded to the digit count of
  the largest line number, followed by two spaces.
- **Highlights** — matched spans rendered in true-inverse colours via
  `theme.Match` (non-current lines) or `theme.CurrentMatch` (current
  matched line, adds underline), applied over the escaped display
  text using the byte→cell maps from the safe-presentation core
  (Issue #7: replaces Issue #5's `theme.Reverse` SGR 7 reverse video
  with explicit inverse colour pairs).
- **Current matched line** — Issue #13: the cursor's current stop's
  line number; its highlights use `CurrentMatch` (true inverse +
  underline). The current matched line follows the cursor, so it
  moves with `n`/`p` navigation.
- **No borders** — no box-drawing border characters around the panel.
- **Layout** — file-list lines padded to the list width, then joined
  with the corresponding content-panel lines.

`renderLineWithHighlights` re-escapes the display text through
`safepresentation.EscapeContent`, then applies `theme.Match` or
`theme.CurrentMatch` to each highlight cell range depending on whether
the line is the current matched line. With the no-style theme, no ANSI
sequences are produced.

### Theme toggle (Issue #7)

The `c` keypress in the browse state calls `theme.Toggle()` on the
model's theme, flipping the composed `View()` styling between the dark
scheme (white on black) and the light scheme (black on white) with no
persistence. `ctrl+c` (cancel) retains global precedence over plain
`c`.

### Cancellation and cleanup

Browse `q` and `ctrl+c` both route through the Issue #4 cleanup path:

- `cancelLoad()` closes `loadCancel`, releasing the file-load goroutine
  from the gate.
- `cancelProcess()` closes the process cancel channel.
- The process boundary kills the child process group and calls
  `proc.Cleanup()`.
- Late `FileLoadCompleteMsg` and `SearchCompleteMsg` are ignored by the
  cancelled model.

## Navigation cursor (Issue #13)

Issue #13 added the single global matched-line cursor that indexes
search stops across files and lines. The cursor is the navigation
anchor; the current file and current matched line both derive from it.
Relevant PRD section: *Navigation, viewport, and logical anchors*.

### SearchIndex cursor

`internal/searchindex` exposes `Cursor`, a circular cursor over an
`Index`'s stops:

- `NewCursor(idx *Index) *Cursor` — creates a cursor that starts at the
  first stop (position 0) when the index has at least one stop, or at
  -1 (no selection) when the index is empty or nil.
- `Stop() (Stop, bool)` — returns the current stop and true, or a
  zero `Stop` and false when the index is empty.
- `Position() int` — the 0-based current stop position, or -1 when
  empty.
- `Len() int` — the number of stops.
- `Next() (Stop, bool, bool)` — advances to the next stop circularly.
  Returns the new stop, whether the cursor moved, and whether the
  file changed (raw path differs). With zero or one stop it is a
  strict no-op: moved and fileChanged are both false.
- `Prev() (Stop, bool, bool)` — retreats to the previous stop
  circularly. Same return contract as `Next`.

The cursor operates on the `Index.stops` slice, which is already
ordered by unsigned raw path bytes then ascending line number and
deduplicated by `(raw path, line number)`. Multiple submatches on one
source line are one stop (the Index merges them). File-change
detection uses `bytes.Equal` on raw paths, so identical paths in
`text` and `bytes` encodings are the same file.

### App wiring

The App creates the cursor on `SearchCompleteMsg` via
`searchindex.NewCursor(m.index)`, replacing the Issue #5 `browseIdx`
field. `handleNavigate(delta)` is called for `n` (delta 1) and `p`
(delta -1):

- With zero or one stop, the cursor is a strict no-op: no state
  change, no load command, no pop-up.
- On a same-file move, only the current matched line styling changes.
  Destination reveal belongs to Issue #14.
- On a cross-file move:
  - The departing file's viewport offset is saved via `saveOffset`.
  - If the destination file is cached in `fileCache`, the panel
    switches immediately and the saved viewport is restored (first
    visit starts at the top).
  - If the destination file is uncached, the panel switches to the
    loading placeholder and `loadFileFor(path)` requests the load.
    The load completes through the existing `FileLoadCompleteMsg`
    path, which caches the buffer and builds the viewport.

`loadFile` now delegates to `loadFileFor` with the cursor's current
stop's raw path. `loadFileFor(path)` is the path-keyed load command
used for cross-file navigation.

### Accessors

- `CursorPosition() int` — the cursor's current position, or -1 when
  empty.
- `CurrentPath() []byte` — the cursor's current stop's raw path, or
  nil when empty.

### Manual scrolling independence

Manual scrolling (Issue #12) does not move the cursor. The cursor
and viewport are independent: scrolling changes only the viewport
offset and per-file saved state, while `n`/`p` change only the
cursor. After manual scrolling, `n`/`p` continue from the last
selected stop, not the manually visible line.

### Passive file list

The file list is passive: there is no direct selection route in
version 1. Keys other than `n`/`p` do not change the current file or
cursor position. The file list underline follows the cursor's current
file, but the user cannot select a file from the list directly.

## Sink-safety method

The hostile-fixture raw-output tests exercise the real composition path
through a no-style theme (`theme.NoStyle()`), which disables all ANSI
sequences. The raw output is checked before any ANSI stripping: no
fixture control byte may survive verbatim in the file list, filename
rule, or panel content.

Fixtures cover: OSC (`\x1b]0;x\x07`), CSI (`\x1b[2J`), C0 controls
(BEL, BS, ESC), C1 (NEL `\xc2\x85`), DEL, standalone CR, invalid UTF-8
path bytes, embedded filename newline, embedded tab, and literal
backslash. Content fixtures add LF, CRLF, and tab.

The `noControlBytes` assertion excludes newlines (which come from the
multi-line layout, not fixtures) but rejects all other C0 controls and
DEL.

## Testing

See [unit-tests](unit-tests.md) for the safe-presentation, FileBuffer,
app browse, and sink-safety test catalogs.
