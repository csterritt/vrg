# Manual vertical scrolling and per-file viewport (Issue #12)

Manual vertical scrolling with per-file saved viewport state, delivered
by
[Issue #12](../issues/012-manual-vertical-scrolling-and-per-file-viewport.md).
Relevant PRD sections: *Navigation, viewport, and logical anchors* and
*Module Design → Viewport / App*. This issue extends the
[Issue #5](browse-tracer.md) browse tracer with scroll units, clamping,
per-file state, and prepared-row rendering.

## Scroll units

Vertical scroll units are rendered rows. Content height is the panel
height minus the filename row (`panelHeight - 1`). Three scroll unit
sizes are supported:

| Key            | Unit       | Amount                                   |
| -------------- | ---------- | ---------------------------------------- |
| `up` / `down`  | one row    | 1                                        |
| `u` / `d`      | half page  | `max(1, floor(contentHeight / 2))`        |
| `pgup` / `pgdn`| full page  | `contentHeight`                          |

Manual scrolling does not move the matched-line cursor. The cursor
stays on its line; only the viewport offset changes.

## Clamping

Scrolling is clamped to valid content extents:

- **BOF clamp** — the top row never goes below 0. Scrolling up at the
  top of the file is a no-op.
- **EOF clamp** — no avoidable blank rows below EOF. The maximum valid
  top row is `max(0, rowCount - contentHeight)`. At this offset, the
  last row is at the bottom of the content area.

Files shorter than the viewport naturally leave unused rows: the
maximum offset is 0, and `Visible()` returns fewer rows than the
content height. The render path does not pad with blank rows.

## Loading placeholder no-op

Scroll keys are no-ops while the content panel shows the `Loading…`
placeholder. The viewport is `nil` until a `FileLoadCompleteMsg` arrives,
so scroll keys are not routed to the viewport. The state stays
`StateBrowse`, no error occurs, and the view still shows `Loading…`.

## Per-file saved state

The model saves the vertical viewport offset per raw path so a file
revisited later can start from its saved position:

- `perFileOffset map[string]int` — keyed by the string form of the raw
  path bytes.
- `currentPath []byte` — the raw path of the currently loaded file.
- `saveOffset()` — called after every scroll action; records the
  current viewport offset for `currentPath`.
- On `FileLoadCompleteMsg`, the saved offset for the loaded file's path
  is restored. A first visit (no saved state) starts at offset 0 (top
  of file).

Issue #13 owns file navigation that triggers revisits; Issue #12
establishes the per-file state mechanism.

## Prepared-row rendering

Frame rendering uses prepared viewport data, not a scan of the full
buffer per frame. Prepared row data (the rendered-row model for the
loaded buffer) is built when a file load completes. Synchronous
preparation is acceptable for this issue; Issue #17 owns
asynchronous/obsolete-layout handling.

### RowProvider interface

`viewport.RowProvider` supplies rendered rows on demand:

- `RowCount() int` — total number of rendered rows available.
- `Rows(start, end int) []filebuffer.Line` — rendered rows for the
  half-open `[start, end)` range.

The Viewport queries only the visible `[offset, offset + contentHeight)`
range. A counting fake can prove the render cost is proportional to the
visible rows, not the full buffer.

### BufferRows adapter

`viewport.BufferRows(buf *filebuffer.Buffer) RowProvider` adapts a
loaded `filebuffer.Buffer` to the `RowProvider` interface. The buffer's
`Lines` slice is the prepared row data; the Viewport slices it for the
visible range only.

### Render-cost guard

The App's render path queries the row provider only for the visible row
range. A `WithRowProviderFactory` option injects a counting fake for
testing: after rendering, only the visible `[offset, offset +
contentHeight)` range should have been queried, never the full buffer.

## Viewport API

`internal/viewport/viewport.go`:

- `New(rows RowProvider, panelHeight int) *Viewport` — creates a
  viewport with the given row provider and panel height. Content
  height is `panelHeight - 1`. The offset starts at 0.
- `ContentHeight() int` — `panelHeight - 1` (0 when `panelHeight ≤ 1`).
- `Offset() int` — the current top row (0-based).
- `SetOffset(int)` — sets the top row, clamped to `[0, maxOffset]`.
- `SetPanelHeight(int)` — updates the panel height and clamps the
  offset (used on resize).
- `SetRows(RowProvider)` — updates the row provider and clamps the
  offset (used when prepared data is rebuilt).
- `Visible() []filebuffer.Line` — queries the row provider for the
  visible range only.
- `ScrollDown()` / `ScrollUp()` — one rendered row.
- `ScrollHalfDown()` / `ScrollHalfUp()` — half a page.
- `ScrollPageDown()` / `ScrollPageUp()` — full page.

## App integration

`internal/app/app.go`:

- `viewport *viewport.Viewport` — the scrollable content view for the
  current file. `nil` while loading or no buffer.
- `perFileOffset map[string]int` — per-file saved vertical offset.
- `currentPath []byte` — raw path of the currently loaded file.
- `rowProviderFactory RowProviderFactory` — test seam for the
  render-cost guard.
- `WithRowProviderFactory(RowProviderFactory)` — option to inject a
  custom row provider factory.
- `ViewportOffset() int` — accessor for the current viewport offset.
- `SavedOffset(path []byte) int` — accessor for the saved per-file
  offset.
- `handleScrollKey(tea.KeyPressMsg) bool` — routes scroll keys to the
  viewport; returns true if handled.
- `saveOffset()` — saves the current offset as per-file state.

### Update flow

On `FileLoadCompleteMsg` with a non-nil buffer:

1. Stores the buffer and clears `loading`.
2. Sets `currentPath` to the loaded file's raw path.
3. Builds a `RowProvider` from the buffer (via the factory or
   `viewport.BufferRows`).
4. Restores the saved per-file offset for the path (0 for a first
   visit).
5. Creates the viewport with the panel height and saved offset.

On `tea.KeyPressMsg` in `StateBrowse` with a non-nil viewport:

1. `handleScrollKey` routes scroll keys (`up`, `down`, `u`, `d`,
   `pgup`, `pgdn`) to the viewport.
2. `saveOffset` records the new offset as per-file state.
3. Unhandled keys fall through to the existing key handlers (`c`, `q`,
   `Esc`).

On `tea.WindowSizeMsg`:

1. Updates `width` and `height`.
2. Calls `viewport.SetPanelHeight(height)` to recompute the layout and
   clamp the offset without losing the reading position.

### Rendering

`renderContentPanel` now queries `m.viewport.Visible()` for the visible
rows only, instead of iterating over `m.buffer.Lines`. The render path
never scans the full buffer per frame. When the viewport is `nil`
(loading or no buffer), the placeholder `Loading…` is shown.

## Testing

See [unit-tests](unit-tests.md) for the viewport and app scroll test
catalogs.
