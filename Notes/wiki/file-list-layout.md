# File-list layout — width formula, truncation, and the visibility toggle

Issue #24
(`Notes/issues/024-file-list-layout-width-truncation-toggle.md`, tasks
`Notes/tasks/024-file-list-layout-width-truncation-toggle.md`) replaced
the provisional file-list width with the real PRD formula, made list
hide/show a first-class user toggle, gave the filename row a
buffer-status slot, and routed every text-width change the layout
introduces through Issue #17's prepared-layout path so the logical
anchor survives each relayout.

PRD cross-references: "File list and layout" (list scrolling, path
truncation, the filename row) and "Layout and indicators" (the width
bullets — the three-term formula, the forty-percent cap, the gutter,
and the reserved indicator column) in `Notes/PRD-vrg.md`.

## The three-term width formula

`Model.listWidth(gutterW, res)` — run on the update path inside
`syncLayout`, its result cached in `listW` so a frame render never
rescans the list — returns the nonnegative minimum of:

1. **Longest sanitized path width plus two** — the list measures the
   `present.Path` display form of every entry (via the `listEntry`
   provider seam), never raw byte lengths, and adds two cells of
   padding.
2. **`floor(0.40 × terminal width)`** — computed as `width*2/5`, so
   odd widths round down (81 columns caps the list at 32).
3. **Terminal width minus the file panel's minimum** — `width −
   (gutterW + 10 + res)`: the gutter (largest line-number digit width
   plus two spaces), ten text cells, and the reserved indicator column
   (`res` is 0 in wrap mode, 1 in run-off-edge — Issue #16's
   `reservedW`; see [wrap-mode.md](wrap-mode.md)).

The result is clamped at zero: the Issue #24 example — `W=20`,
gutter 9, reserved 1 — yields `20 − (9+10+1) = 0`, and pathological
dimensions can never produce a negative list or panel width.

Recalculation happens wherever the terms can move: terminal resize,
the `w` wrap/mode change (which flips `res`), a load completing with a
different gutter width, search completion, a file crossing, and the
list's own hide/show toggle — all inside `syncLayout`, which also
resizes the viewport and issues the current file's layout request when
its installed rows are missing or stale-keyed.

## Terminal, panel, and text widths

Issue #38 (`Notes/tasks/038-viewport-content-panel-width.md`) pinned
the three-width chain the panel layout is built on and that every
viewport install site must honour:

- **Terminal width** — `m.width`, the frame's full cell count.
- **Panel width** — the terminal width minus the file list's
  allocated cells (`m.width − m.listW`): the columns right of the
  list block. The list's own `longest + 2` padding supplies the
  visual separation; the layout draws no separate divider column.
- **Text width** — the panel width minus the buffer gutter
  (`gutterDigits() + 2`) minus the reserved right-indicator column
  (`reservedW()`: one in run-off-edge mode, zero in wrap). This is
  `m.textW`, the width `layoutKey` carries and `Viewport.Resize`
  installs.

The single computation lives in `syncLayout` — `panelW := max(0,
m.width-m.listW)` then `m.textW = max(0, panelW-gutterW-res)` — and
governs everything downstream: `vp.Resize(m.textW, …)`, `layoutKey`'s
`Width` (which the `layoutDoneMsg` and `currentRows` install guards
compare against), the wrap row packing in `viewport.Prepare`,
run-off-edge clipping, `,`/`.`/`<`/`>`/`[`/`]` panning and the
visible-lines extent clamp, the minimal horizontal reveal, and both
hidden-content indicators — the gutter `_`/`*` at the list-derived
column and the reserved `*` at the panel's right edge, outside the
text area. Resize and the list's hide/show re-measure through the
same path, so pan and reveal state always tracks the current panel
width. Issue #38's tests compute their expected positions from this
chain (`wantTextW()`) rather than the cached `m.textW`, so a
regression deriving the viewport width from the raw terminal width —
or dropping the gutter or reserved terms — fails.

## Requested-visible versus allocated width

`listShow` is the user's visibility preference — shown initially,
`left`/`tab` hide, `right`/`shift+tab` show, both browse-phase keys
that no-op when repeated. The formula gates on it: a hidden list
returns width 0. The preference is never automatically changed by the
computed width — a zero-width *allocation* (term 3 binding) draws no
list cells while `listShow` stays true, so the list reappears on its
own when the geometry allows.

Both transitions are text-width changes: the press runs `syncLayout`,
which recomputes `textW`, resizes the viewport, and returns the keyed
relayout request — preparation off the update path, installation only
on a matching key, the retained logical anchor resolving against the
new row model (see
[logical-anchor-and-layout.md](logical-anchor-and-layout.md)). A
`tab`/`shift+tab` round trip leaves the top of the panel showing the
same text — including mid-way through a wrapped line. The same
mechanism covers gutter growth: a load completing with wider line
numbers narrows `textW`, issues the keyed request, and the anchor's
text stays at the top in both directions of the round trip.

## Grapheme-safe left truncation

`present.TruncateLeft` — Issue #24's `truncateLeft`, promoted into the
shared cell-width helper by Issue #39 — keeps the rightmost cells of an
over-wide entry or path within the budget, marking truncation with a
leading `…` so basenames stay visible. The cut lands only on a grapheme
boundary: a cluster straddling it drops whole rather than splitting, so
the result never exceeds the budget and never shows half a glyph (the
PRD's wide and combining characters). `ansi.TruncateLeft` alone could
not guarantee this: it keeps a wide cluster straddling the cut,
overflowing the budget by a cell. The shared form is also ANSI-aware —
escape sequences paint no cells and survive the cut — while
`listWidth`'s longest-entry measure and the render loop's padding use
`present.CellWidth`, so wide and combining names count by the cells
they paint.

## The filename row's status slot

`filenameRule(path, note, w)` embeds the escaped current path in the
horizontal rule and reserves a **buffer-status note slot** between the
path and the trailing dash run. The note wins cells over the path: an
oversized path left-truncates — down to nothing — so the note paints
whole; a note too wide even for an empty path is dropped rather than
clipped mid-text. Nothing overflows the panel width and degenerate
widths fall back to dashes. `Model.statusNote` is the provider seam;
tests drive it with a synthetic string while the production value
comes from `bufferNote` — Issue #26 delivered the first real note,
`(unreadable)` for a read-failed path (see
[read-failures.md](read-failures.md)), Issue #29 added `file changed
since search` for a stale-validating buffer (see
[stale-match-validation.md](stale-match-validation.md)), and Issue #30
added `(unsupported encoding)` for a BOM-classified buffer (see
[unsupported-encodings.md](unsupported-encodings.md)).

## Minimal-movement list scrolling

`listTop` is the file list's scroll offset — the index of the first
visible entry — and `scrollList` adjusts it inside `syncLayout` with
minimal movement: the window shifts only when the current file's entry
falls outside it, so entries below the active one stay put while
retreating. Rendering still paints only the scrolled window: the
`listEntry` provider is queried once per visible row
(`TestRenderQueriesOnlyVisibleWindow` proves the queried slice is
exactly `files[listTop:listTop+height]` deep into a long list — the
Issue #17 render-cost guarantee; see
[logical-anchor-and-layout.md](logical-anchor-and-layout.md)).

## Tests

- `internal/app/filelist_test.go` — `TestListWidthFormula` table-drives
  every term of the formula (each winning, the `floor` rounding, the
  ten-cell panel minimum, gutter growth, zero and pathological widths);
  `TestTruncateLeftGraphemeSafe` pins the leading-`…` cut at grapheme
  boundaries for wide, combining, and ZWJ clusters;
  `TestFilenameRuleStatusSlot` covers the note slot's priority,
  truncation, and drop rule; `TestListToggleKeys` /
  `TestListToggleKeysInertWhileSearching` cover the shown-initially
  state, all four keys, the keyed relayout request per press, and the
  repeated-press no-op; `TestListHideShowPreservesAnchorText` and
  `TestGutterGrowthRelayoutKeepsAnchorText` prove the anchor's text
  stays at the top through hide/show and gutter-growth round trips;
  `TestListReducedLeavesTenTextCells` drives a real five-digit-gutter
  file at 30 columns into run-off-edge mode; `TestZeroWidthAllocationKeepsPreference`
  covers the `W=20`/gutter 9/reserved 1 zero-allocation case;
  `TestListScrollsToKeepActiveVisible` pins minimal-movement
  scrolling; `TestListRenderQueriesOnlyVisibleWindow` asserts the
  exact queried slice; `TestStatusSlotRendersInFilenameRow` renders
  the synthetic note at 80 and 30 columns.

See [unit-tests.md](unit-tests.md) § `internal/app`.

## Files

- `internal/app/app.go` — `listShow`/`listTop` fields, the
  `statusNote` provider seam, the `left`/`tab` and `right`/`shift+tab`
  browse-key cases.
- `internal/app/browse.go` — `listWidth` (the formula and the
  `listShow` gate), `scrollList`, `syncLayout`'s scroll and geometry
  sequencing, `filenameRule`'s note slot, `renderBrowse`'s `listTop`
  window — width and truncation routed through `present.CellWidth`/
  `present.TruncateLeft` since Issue #39.
- `internal/present/cellwidth.go` — `TruncateLeft`'s cluster-boundary
  cut, shared with every other truncation consumer.

See also: [browse-tracer.md](browse-tracer.md) (the two-pane
composition this lays out),
[logical-anchor-and-layout.md](logical-anchor-and-layout.md) (the
prepared-layout path every width change routes through),
[wrap-mode.md](wrap-mode.md) (the reserved indicator column in term 3),
[safe-presentation.md](safe-presentation.md) (the escaped paths being
measured and truncated), and
[match-navigation.md](match-navigation.md) (the cursor whose file the
list follows).
