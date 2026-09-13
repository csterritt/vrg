# Vertical destination reveal (Issue #14)

Vertical destination reveal, delivered by
[Issue #14](../tasks/014-vertical-destination-reveal.md). Relevant PRD
section: *Navigation, viewport, and logical anchors* in
[PRD-vrg.md](../PRD-vrg.md). This issue extends the
[Issue #12](manual-vertical-scrolling.md) viewport and the
[Issue #13](browse-tracer.md) navigation cursor with destination-reveal
behavior: navigating to a match adjusts the viewport so the rendered
row containing the match's display target is visible, without
unnecessary scrolling.

## Display target

The display target is the **start cell of the first submatch on the
destination line** (the marker cell for a zero-width match), subject to
stale-entry fallback. Submatches in a `searchindex.Stop` are ordered by
byte start then end, so the first submatch identifies the target.

The reveal targets the **rendered row containing this target**, not
merely the source-line ordinal. Issue #16 completed the wrapped
mapping: `targetRow(stop)` uses `RowModel.RowFromByte(lineIndex,
byteOffset)` with the first submatch's byte start to find the wrapped
row containing the match. In run-off-edge mode (or when the row model
is unavailable), it falls back to the 0-based source line index
(`stop.LineNumber - 1`). A source line taller than several screens
still reveals the match at `floor(contentHeight / 3)`. See
[wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md).

`Model.targetRow(stop searchindex.Stop) int` returns the 0-based
rendered row for a stop.

## Visible-target no-scroll

If the target row is already within the visible range
`[offset, offset+contentHeight)`, the viewport does not scroll. This
avoids unnecessary scrolling when navigating between on-screen
matches.

## One-third placement

If the target row is hidden, the viewport is moved so the target lands
at zero-based row `floor(contentHeight / 3)`. The new top offset is
`targetRow - floor(contentHeight / 3)`, clamped to valid top positions.

## BOF and EOF precedence

At BOF and EOF, available content takes precedence over one-third
placement. The computed offset is clamped to `[0, maxOffset]`, so:

- **BOF clamp** — a target near the top stays near the top. The
  offset clamps to 0 rather than going negative, so the target appears
  at its natural position (e.g. row 1) instead of being forced down to
  the one-third row.
- **EOF clamp** — a target near the bottom stays near the bottom. The
  offset clamps to `maxOffset = max(0, rowCount - contentHeight)`, so
  the target appears at `targetRow - maxOffset` (e.g. row 7) instead
  of being forced up to the one-third row.

## Saved viewport versus first-visit starting sequence

On a file change, the starting viewport is determined before the
reveal:

- **Revisit** — the saved per-file vertical offset is restored as the
  starting point.
- **First visit** — the starting offset is 0 (top of file). This
  includes the startup file's first load.

The reveal is then applied from that starting point. A first visit
(including the startup file) therefore begins from the top before the
reveal adjusts.

## Reveal triggers

The reveal is applied at:

- **Startup after load** — when the startup file's first load completes,
  the first match is revealed. Issue #14 owns this startup-after-load
  trigger; Issue #28 owns the broader load-completion reveal contract
  that defers the reveal to the matching layout installation.
- **Same-file navigation** — `n`/`p` within the same file reveals the
  new target row.
- **Cross-file navigation to a cached destination** — the cached
  destination is shown with its saved viewport restored, then the
  reveal is applied.
- **Cross-file navigation to an uncached destination** — the reveal is
  applied when the load completes and the matching layout installs.

Reload (`r`) does not trigger a reveal. The reload sets the
`IntentReloadAnchor` intent, so a reload preserves the saved viewport
anchor without revealing a match (PRD: "Reload by itself does not
reveal a match"). See [explicit-reload](explicit-reload.md) and
[load-completion-two-stage](load-completion-two-stage.md).

## Reveal's effect on saved state

A reveal that moves the viewport **replaces** the saved per-file
vertical state: the new offset is saved as that file's current saved
state. A no-scroll reveal (target already visible) **does not
discard** the retained saved state.

`Model.revealTarget()` records the offset before the reveal, calls
`viewport.Reveal(targetRow)`, and saves the new offset via
`saveOffset()` only if the offset changed.

## Viewport.Reveal API

`internal/viewport/viewport.go`:

- `Reveal(targetRow int)` — adjusts the viewport offset so the target
  row is visible. If the target is already within the visible range,
  the offset is unchanged. Otherwise the viewport is moved so the
  target lands at zero-based row `floor(contentHeight / 3)`, clamped to
  `[0, maxOffset]` (BOF/EOF precedence).

## App integration

`internal/app/app.go`:

- `loadIntent LoadIntent` — carries the pending intent from stage one
  (load completion) to stage two (matching layout installation). Issue
  #28 generalizes the earlier `needsReveal`/`pendingReveal`/`pendingReloadAnchor`
  flags into this single field. See
  [load-completion-two-stage](load-completion-two-stage.md).
- `revealTarget()` — reads the cursor's current stop, computes the
  rendered target row, records the offset, calls `Reveal`, and saves
  the new offset only if the reveal moved.
- `commitLoadIntent()` — Issue #28 stage two: commits the pending
  intent against the installed rows. `IntentReveal` calls
  `revealTarget()`; `IntentReloadAnchor` preserves the anchor (no
  reveal); `IntentNone` is a no-op. The intent is cleared after commit.
- `targetRow(stop)` — returns the 0-based rendered row for the stop's
  first submatch start cell.

### Update flow

On `SearchCompleteMsg` entering `StateBrowse`:

1. Creates the cursor (Issue #13).
2. Sets `loadIntent = IntentReveal` for the startup load.
3. Requests the startup file load.

On `FileLoadCompleteMsg` with a non-nil buffer (Issue #28 stage one):

1. Stores the buffer, caches it, sets `currentPath`.
2. Builds the row provider and restores the saved offset (0 for a
   first visit).
3. If the layout installed synchronously (cache hit or test seam),
   commits the intent immediately. Otherwise the intent is carried to
   stage two.

On `LayoutReadyMsg` with a matching key (Issue #28 stage two):

1. Installs the prepared `RowModel`, preserves the anchor for
   same-file rebuilds, restores the saved per-file anchor for fresh
   loads.
2. Commits the pending intent via `commitLoadIntent()`.

On same-file `handleNavigate`:

1. Applies `revealTarget()` to the new cursor stop. If a layout is
   pending, carries `IntentReveal` for the next installation.

On cross-file `handleNavigate` to a cached destination:

1. Saves the departing file's offset.
2. Switches the panel, restores the saved offset (0 for first visit).
3. Sets `loadIntent = IntentReveal`. Commits immediately on a cache
   hit; carries the intent on a stale-cache miss.

On cross-file `handleNavigate` to an uncached destination:

1. Saves the departing file's offset.
2. Sets `loadIntent = IntentReveal`.
3. Requests the load. The reveal is applied when the matching layout
   installs.

## Issue boundaries

- **Horizontal reveal** is owned by Issue #19. Issue #14 only handles
  vertical reveal.
- **Load-completion reveal** in general is owned by Issue #28. Issue
  #14 owns only the startup-after-load trigger. See
  [load-completion-two-stage](load-completion-two-stage.md) for the
  two-stage contract that defers reveal decisions to the matching
  layout installation.
- **Wrapping** (mapping a display cell to a sub-row within a wrapped
  source line) is owned by Issue #16. Until then, `targetRow` returns
  the 0-based source line index.

## Testing

See [unit-tests](unit-tests.md) for the viewport reveal and app
reveal test catalogs.
