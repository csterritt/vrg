# Logical anchor and prepared layouts (Issue #17)

Delivered by
[Issue #17](../issues/017-logical-anchor-through-rewrap-and-resize.md)
([task](../tasks/017-logical-anchor-through-rewrap-and-resize.md)):
the file panel's reading position becomes a **width-independent logical
anchor** — `(source line, display-column offset)` — retained through
rewraps, wrap toggles, and resizes, and every `viewport.Prepare` row
model is built **off the update path** as a Bubble Tea command whose
keyed completion installs only while it still matches the live layout.
Relevant PRD sections: *Navigation, viewport, and logical anchors*
(every anchor bullet) and *Module Design → Viewport* (the
retain/restore-anchor interface) in [`Notes/PRD-vrg.md`](../PRD-vrg.md);
user stories 62–63. Builds on
[viewport-scrolling.md](viewport-scrolling.md) (the scroll units and
per-file viewport the anchor rides on),
[wrap-mode.md](wrap-mode.md) (the keyed swappable `Rows` this issue
moves off `Update`), and
[destination-reveal.md](destination-reveal.md) (the reveal whose
pending intents this issue adds).

## The logical anchor (`internal/viewport`)

`Anchor{Line, Cell}` is the retained reading position: the 1-based
source line plus the display-column offset within that line of the
location the effective top row must contain. Unlike a rendered-row
ordinal it means the same text under any wrap width or mode — a 95-cell
line scrolled to its sixth row at width 10 anchors `{Line: 1, Cell:
50}`, and at width 7 that same anchor restores to the row holding cell
50 (row 7), not the sixth row of the new layout.

`Viewport` now carries both `top` (the effective first visible rendered
row) and `anchor` (the retained logical position), and every
positioning operation takes a `Model` — the new interface abstracting
the prepared rows:

```go
type Model interface {
	Len() int
	AnchorAt(row int) Anchor // the location a rendered row starts at
	RowOf(a Anchor) int      // the rendered row containing a location
}
```

`*Rows` implements it: `AnchorAt` returns the row's source line number
and first display cell; `RowOf` finds the line's wrapped rows via
`firstRow` and returns the one whose span holds the cell (a past-end
cell lands on the line's last row; an out-of-range line clamps to the
nearest real row; the empty model maps everything to row 0). App tests
substitute fakes through the same interface.

## Anchor replacement rules

The anchor is *retained state*, replaced only by deliberate movement:

- **`Scroll`** — a scroll that moved the effective top replaces the
  anchor with the resulting top row's `AnchorAt` location; a scroll
  clamped to a no-move leaves a retained mid-row column intact.
- **`Reveal`** — a reveal that moves the viewport replaces the anchor
  the same way; a **no-scroll reveal keeps it**, so a target already on
  screen never discards a retained logical column.
- **`Restore(m, height)`** — the rewrap path: the new effective top is
  `m.RowOf(anchor)` — the row *containing* the retained location, never
  the row with the same former ordinal. Wrap-off keeps the column while
  showing the line as one row; wrapping back restores the row holding
  it.
- **EOF clamping is intentionally lossy.** When clamping pulls the
  effective top upward — inside `Scroll`, `Clamp`, or `Restore` — the
  anchor is rewritten to the clamped top row's location. A later shrink
  does not resurrect the pre-clamp position: the file kept no avoidable
  blank rows, and the old top is gone on purpose (PRD user story 63).
  `reanchor` is the single private helper: it updates the anchor iff
  the effective top changed under a non-empty model.

## Prepared layouts off `Update` (`internal/app`)

`viewport.Prepare` is O(file), so Issue #17 makes it a command:

- `requestLayout(path)` returns a `tea.Cmd` running the whole
  `viewport.Prepare` under `options.layoutGate` — preparation never
  runs inside `Update`. It returns nil on the **fast path** (the
  installed model already matches the live key — including test fakes,
  which are always taken as current), when an identical request is
  already in flight, or when no buffer is cached yet.
- `layoutKey(path, buf)` is the live demand:
  `Key{Path, Revision, TextWidth, Wrap}` — the raw path, the per-path
  content revision (`m.revs` bumps on every successful load), the text
  width under the buffer's gutter, and the wrap mode.
- `layoutReqs map[string]viewport.Key` records the **newest** in-flight
  key per path, deduplicating requests; a second resize or `w` while a
  worker is gated replaces the recorded key so the superseded
  completion is obsolete on arrival.
- `layoutReadyMsg{key, rows}` delivers the result. Update installs it
  only when the buffer is still cached **and** `msg.key` still equals
  `layoutKey(path, buf)` — otherwise it is **discarded without
  touching** installed rows, saved viewports, anchors, or pending
  intents. Out-of-order and rapid-toggle completions die this way; a
  completion minted under a superseded revision does too.
- `currentRows(path)` is the only prepared data rendering, scrolling,
  and reveals may consult: an installed `*Rows` whose key no longer
  matches the live layout is **invisible** — the panel shows
  "Loading…" until its replacement installs, and scroll/reveal on it
  are no-ops. Stale layouts can never be rendered or mutate state.

Three triggers issue requests: `fileLoadedMsg` (after caching the
buffer and bumping its revision), `WindowSizeMsg` (the new text width
re-keys every layout; the current file's replacement is requested and
its retained anchor — not its row ordinal — carries the position into
it), and the `w` toggle (which flips `m.wrap` at once and requests the
re-keyed layout — run-off-edge reserves one indicator column, so the
text width itself changes). `navigate` also requests the destination's
layout so a **cached file with a stale layout gets a fresh preparation**
rather than rendering obsolete rows.

## Pending reveal intents

`model.reveal` can no longer assume a usable layout: when
`currentRows` is nil — no model yet, or a stale one hidden pending its
replacement — the reveal **pends** on `pendingReveals[path]` instead of
running. When a matching completion installs for the still-current
file, the intent commits against the fresh layout; the same branch
covers a first visit's initial reveal (`!had` — no model was ever
installed). Rapid `n`/`p` presses while a worker is gated leave only
the **latest** intent: a single bool per path, overwritten each time.
`navigate` itself still runs the full sequence — cursor step, reveal
attempt, `startLoad`, pop-up — synchronously, so input stays actionable
no matter how long preparation takes.

## Render-cost guarantees

PRD *Resources and responsiveness* bounds both panes' per-frame work:

- The frame render still queries `rowSource` only for the visible
  range `[top, top + content height)` — `contentCell` slices prepared
  spans; `View()` never wraps or rescans the buffer (the
  `countingRows` fake test stands). Issue #18 extends the same bound
  horizontally: `MaxOff` extent evaluation queries only the visible
  rows of the prepared layout (the `countingExtent` fake proves it),
  and it runs on the write paths — pans and visible-set changes —
  never inside `View()`.
- The file list is equally bounded: `listWBase` — the longest escaped
  path width plus one — is computed **once** at `searchDoneMsg`, and
  `listCell` escapes only the visible window's entries through
  `m.escapePath` (the `options.escapePath`/`WithEscapePath` seam counts
  provider queries in tests). `filenameRule` escapes only the current
  path.

## Tests

`internal/viewport/anchor_test.go` (external package): rewrap keeping
the text location across two widths, the wrap-off/wrap-on round trip
preserving the logical column, scroll replacing the anchor (with a
clamped no-move leaving it), a moving reveal replacing it, a no-scroll
reveal retaining a mid-row column, and both lossy-clamp cases — `Clamp`
pulling the top up and `Restore` doing the same under a grown
viewport. `internal/app/anchor_test.go` drives a resize through the
model to prove the cursor selection and retained anchor both survive.
`internal/app/layout_test.go` covers the async pipeline: the gated
worker keeping `n`/`p`/`w`/resize/`q` responsive, `ctrl+c` exiting 130
mid-preparation, out-of-order W1→W2→W3 completion order, rapid wrap
toggles discarding superseded layouts, an obsolete completion for a
non-current file leaving all state untouched, the superseded-revision
discard, the cached-file stale-layout re-request and matching-layout
fast path, the pending reveal committing on install, and the
file-list escape-count bound. Issue #18 widened `Model` into
`Extent` (`Key`/`At` added) for the operations that re-clamp the
horizontal offset — see
[horizontal-panning.md](horizontal-panning.md). See
[unit-tests.md](unit-tests.md).
