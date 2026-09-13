# Horizontal panning (Issue #18)

Issue #18 adds horizontal panning in run-off-edge mode: `,`/`.` pan one
column, `<`/`>` pan ten columns, `[`/`]` pan half the text-area width.
Panning is a no-op in wrap mode. The horizontal offset is separate from
the vertical offset, retained through wrap toggles, re-clamped on every
return to run-off-edge mode and on every visible-set change, and reset
to zero on file change. Split grapheme clusters at the clip edge render
as blank cells rather than partial glyphs.

This page cross-references:

- [wrap-mode-and-grapheme-policy](wrap-mode-and-grapheme-policy.md) —
  wrap/run-off-edge modes, the reserved indicator column, and the
  shared grapheme segmentation policy that clipping consumes.
- [logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md)
  — the prepared-layout architecture and visible-set changes that
  trigger horizontal re-clamping.
- [manual-vertical-scrolling](manual-vertical-scrolling.md) — the
  vertical offset and per-file saved state that horizontal state
  parallels.
- `Notes/PRD-vrg.md` — Navigation, viewport, and logical anchors
  section; Layout and indicators section.

## Pan units

| Key | Unit |
|-----|------|
| `,` | one column left |
| `.` | one column right |
| `<` | ten columns left |
| `>` | ten columns right |
| `[` | half text width left: `max(1, floor(textWidth / 2))` |
| `]` | half text width right: `max(1, floor(textWidth / 2))` |

`Viewport.HalfPanWidth` returns `max(1, floor(textWidth / 2))`. The app
routes pan keys via `handlePanKey`, which calls `Viewport.Pan(columns)`.
`Pan` is a no-op in wrap mode; in run-off-edge mode it adds the shift
and clamps to the paintable-boundary maximum.

## Three distinct width definitions

Issue #18 keeps three definitions separate to avoid confusion:

1. **Content extent** — the effective display width of a single line,
   the sum of its cluster widths. Future Issue #23 end-of-line marker
   cells will extend this; the Issue #18 API treats the extent as
   forward-compatible but does not implement marker behavior.
2. **Extent policy** — the viewport computes the maximum from the
   widest currently rendered source line only, not the whole file. A
   long line scrolled out of view does not contribute.
3. **Maximum valid offset** — `max(0, S)` where `S` is the largest cell
   index at which a grapheme cluster (or future marker) of the widest
   visible line starts and its cell width fits within the text width.

## Paintable-boundary maximum

`Viewport.MaxHOffset` returns the paintable-boundary maximum for the
current visible rows:

- At the maximum, at least one whole grapheme cluster (or future
  marker cell) is fully painted whenever any cluster fits the text
  width.
- A line ending in a two-cell cluster stops at that cluster's start
  (`E - 2`), not at the final cell, so both cells remain visible at the
  maximum.
- If the final cluster is wider than the text width, the maximum falls
  back to the last fitting cluster's start.
- If no cluster fits, the maximum is zero; clipping blanks are
  intentional for the unpaintable portion.
- An empty buffer, placeholder, or all-empty visible view clamps to
  zero.

The maximum is recomputed on every pan and on every visible-set
change; it is never cached across operations.

## Visible-set re-clamping

The horizontal offset is re-clamped to the new maximum whenever the
visible line set changes:

- vertical scrolling (`ScrollDown`/`Up`/`HalfDown`/`HalfUp`/`PageDown`/`PageUp`)
- destination reveal (`Reveal`)
- resize (`SetPanelHeight`, `WindowSizeMsg`)
- file-list hide/show and gutter growth (text-width-only `SetLayout`)
- wrap-toggle re-entry (see below)

If a long line scrolls out of view and short lines become visible, the
offset is destructively clamped leftward. Returning to the long line
does not restore the former offset; the clamp is intentionally lossy,
mirroring the Issue #17 vertical EOF-clamp policy.

## Retention through wrap toggles with re-entry clamping

The horizontal offset is retained while in wrap mode (panning is a
no-op and the offset is not used for rendering). On every return to
run-off-edge mode the offset is re-clamped against the current visible
rows' maximum.

`Viewport.SetLayout(textWidth, wrapMode)` distinguishes two cases:

- **Wrap-mode change** (wrap toggle): the clamp is deferred to the next
  `SetRows` call so it uses the new rows; the offset is carried over
  unmodified until then.
- **Text-width-only change** (list hide/show, gutter growth): the clamp
  runs immediately against the current visible rows.

`Viewport.clampHOffset` is a no-op in wrap mode, so the offset is
preserved while wrap is on. `Viewport.SetHOffset` (used by the app to
carry the offset across rebuilds) sets the offset and clamps it in
run-off-edge mode; in wrap mode it stores the offset without clamping.

## Reset on file change

`Viewport.ResetHorizontal` sets the offset to zero. The app calls it
on file change (fresh load or cross-file navigation) before any
horizontal reveal, so a newly opened file starts at offset zero.

## Grapheme-safe clipping

`Viewport.ClipLine(line)` clips a source line to the visible horizontal
window `[hOffset, hOffset + textWidth)` with grapheme-safe blank cells:

- A cluster split by the left or right clip edge is replaced with blank
  cells for its visible portion, so no half glyph is drawn.
- At the maximum, a fitting cluster remains fully painted.
- The exception is a cluster wider than the text area, where the
  in-window portion may intentionally render as blanks.
- Highlights are shifted by `-hOffset` and clamped to `[0, textWidth)`.
- In wrap mode the line is returned unchanged.

The app's `renderContentPanel` calls `ClipLine` on each visible row
before `renderLineWithHighlights`, so clipping integrates with the
existing true-inverse match styling.

## Render-cost guard

Extent computation inspects only the prepared layout's currently
visible rows (`Viewport.Visible` range), not the full buffer. This
mirrors the Issue #12 visible-range render-cost guard: the maximum is
derived from `RowProvider.Rows(offset, offset + contentHeight)` and
the widest line among those rows. No scan of the whole file occurs
during panning or re-clamping.

## App integration

- `handlePanKey` routes `,`/`.`/`<`/`>`/`[`/`]` to `Viewport.Pan`.
- `buildViewport` (synchronous bypass, cache hit, and `LayoutReadyMsg`
  paths) carries over the horizontal offset on same-file rebuilds and
  calls `ResetHorizontal` on file change.
- `SetLayout` is called on every viewport (re)installation so the
  text width and wrap mode are current.
- `WindowSizeMsg` updates the text width via `SetLayout` when the
  factory test seam is in use.
- `ViewportHOffset()` exposes the offset for tests and inspection.
