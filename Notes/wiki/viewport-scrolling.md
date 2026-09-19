# Manual vertical scrolling and per-file viewport state (Issue #12)

Delivered by
[Issue #12](../issues/012-manual-vertical-scrolling-and-per-file-viewport.md):
manual vertical scrolling of the current file's panel — `up`/`down` by
one rendered row, `u`/`d` by a half page, `page up`/`page down` by a
full page — clamped to valid content, with per-file saved viewport
state and a frame render that slices prepared row data instead of
rescanning the buffer. Relevant PRD sections: *Navigation, viewport,
and logical anchors* (the scroll-unit bullet and the EOF-clamp bullet)
and *Module Design → Viewport* in [`Notes/PRD-vrg.md`](../PRD-vrg.md);
user stories 60–61. See also [browse-tracer.md](browse-tracer.md) for
the Issue #5 frame this extends.

## Scroll units (`internal/viewport`)

`Viewport` is the file panel's vertical window over one loaded file's
prepared rows; the zero value shows the top of the file and `Top()` is
the first visible rendered row. All scroll units are **rendered rows**
over the **content height** — the file-panel height minus the filename
row (`model.contentRows()` = frame height − 1):

| Keys | Unit | Formula |
|---|---|---|
| `up` / `down` | one rendered row | ±1 |
| `u` / `d` | half page | ±`max(1, floor(height / 2))` — `viewport.HalfPage` |
| `pgup` / `pgdown` | full page | ±content height |

`Viewport.Scroll(d, rows, height)` moves the top row and clamps.
`Viewport.Clamp` re-applies the bound after the row count or content
height changes, and `MaxTop(rows, height)` is the largest valid top:

- **BOF clamp** — top never goes below 0; `up` at the top of the file
  does nothing.
- **EOF clamp** — top never exceeds `max(0, rows − height)`, the last
  position leaving no avoidable blank rows below EOF; the file's last
  line lands on the panel's bottom row and further scrolling is a
  no-op. A file no taller than the viewport — or a zero content height —
  clamps to 0: files shorter than the viewport naturally leave their
  unused rows blank, which is not overscrolling. Shrinking content or a
  growing viewport pulls a stranded top upward — the PRD's intentionally
  lossy EOF clamp (the logical-anchor contract is Issue #17's).

## App wiring (`internal/app`)

- `model.vps map[string]viewport.Viewport` is the **per-file saved
  vertical state**, keyed by raw path like the other browse caches.
  Scrolling writes through to it on every keypress, and Issue #14's
  moving reveals replace it too, so a file revisited later through
  Issue #13's `n`/`p` navigation starts the destination reveal from its
  last position — the handoff needs no explicit save — and a
  never-visited file's zero value starts at the top. `Update` re-clamps
  every prepared file's saved viewport on `WindowSizeMsg` and re-clamps a
  file's saved entry when its load completes.
- Scroll keys are browse-state only (`isScrollKey` in `scrollBy`), so the
  open overlay keeps its own `up`/`down` handling and all other keys
  ignored, and the searching/no-results states are unaffected. Manual
  scrolling never moves the matched-line cursor — Issue #13's `n`/`p`
  therefore continue from the last selected stop; see
  [match-navigation.md](match-navigation.md).
- **Placeholder no-op** — while the panel shows "Loading…" or
  "(unreadable)" there is no row model for the file and `scrollBy`
  returns without creating viewport state.
- **Prepared-row rendering** — `fileLoadedMsg` builds
  `viewport.Prepare(buf)` into `model.rows`, the rendered-row model for
  the loaded buffer at the current layout. In the still-unwrapped panel
  each source line is one rendered row, so the model is the buffer's
  lines plus its gutter width and width changes do not rebuild it;
  Issues #16/#17 own wrap-aware row models keyed by
  (path, revision, text width, wrap mode) and preparation off the update
  path. `contentCell` renders the row at `top + content row` via the
  `rowSource` seam, so a frame queries only the visible range
  `[top, top + content height)` — never an O(N) scan of the buffer
  (PRD *Resources and responsiveness*).

## Tests

`internal/viewport/viewport_test.go` (external package) covers each
scroll unit's movement and symmetric return, the `HalfPage` formula over
odd and degenerate heights, the BOF/EOF clamps over files shorter than,
equal to, and longer than the viewport (including empty and zero-height),
the lossy `Clamp` on growth/shrink, and `Prepare`'s row model over a real
buffer plus the nil-buffer empty model.

`internal/app/scroll_test.go` (same package; Issue #12) drives keys
through `Update`: every unit's movement and its rendered first content
row, the EOF stop with the last line on the bottom row, `up`/`u`/`pgup`
at BOF doing nothing, a shorter-than-viewport file leaving naturally
blank rows, scroll keys as no-ops on both the "Loading…" and
"(unreadable)" placeholders (no viewport state created), per-file state
surviving a simulated leave-and-revisit while the second file keeps its
own top-of-file state, the counting-fake `rowSource` proving a frame
queries exactly the visible row range, and the resize re-clamp. See
[unit-tests.md](unit-tests.md).
