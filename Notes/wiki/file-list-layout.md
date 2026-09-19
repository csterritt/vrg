# File-list layout — width formula, truncation, and visibility toggle (Issue #24)

Delivered by
[Issue #24](../issues/024-file-list-layout-width-truncation-toggle.md)
([task](../tasks/024-file-list-layout-width-truncation-toggle.md)):
the file list sizes itself responsively instead of holding a fixed
width — a three-term minimum with floor rounding, grapheme-safe
left-truncation with a leading `…` for paths that outgrow their
cells, `left`/`tab` and `right`/`shift+tab` hide/show controls with
the list initially shown, auto-scrolling that keeps the active entry
visible, a filename-row status-note slot the path truncates to make
room for, and logical-anchor preservation through every relayout the
width changes cause. Relevant PRD sections: *File list and layout*
(user stories 24–30) and *Layout and indicators* (the terminal
minimum, the width-formula bullet, the zero-width bullet, the
truncation/auto-scroll/filename-row bullet, and the
Layout/indicators acceptance row) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). Builds on
[logical-anchor.md](logical-anchor.md) (the keyed prepared-layout
pipeline every text-width change routes through),
[wrap-mode.md](wrap-mode.md) (the reserved indicator width the
formula subtracts), [browse-tracer.md](browse-tracer.md) (the
two-pane frame and filename rule), and
[file-change-popup.md](file-change-popup.md) (the shared
`leftTruncate`/`tailCells` helpers).

## The width formula

`fileListWidth(longest, width, gutter, resInd)` computes the
nonnegative minimum of three terms:

- `longest + 2` — the longest sanitized path's display width plus
  two cells of padding;
- `width * 2 / 5` — `floor(0.40 × terminal width)`, integer
  arithmetic doing the flooring (at 80 columns the cap is 32);
- `width − gutter − 10 − resInd` — the terminal width minus the
  file panel's reservation: the line-number gutter, a ten-cell
  minimum text width, and the reserved right-indicator width (one
  cell in run-off-edge mode, zero in wrap).

The third term is the only one that can drive the result to zero —
minimum text width takes precedence over the 40% cap — and the
final clamp returns 0 rather than a negative width.

`listWidthFor(gutter)` applies the formula for a given gutter;
`listWidth()` supplies the current file's gutter via
`gutterWidth()` — the installed row model's, else the cached
buffer's, else 3 — so the list always matches the gutter a rendered
frame would show. `textWidth(gutter)` subtracts list, gutter, and
reserved widths from the terminal width and clamps to zero, so no
dimension ever goes negative.

`m.listWBase` holds the longest sanitized path width, computed once
at `searchDoneMsg` from `escapePath` output — frame rendering never
rescans the file list.

None of this runs below the 20×3 terminal minimum: the Issue #33
gate replaces the frame and skips layout work entirely — see
[terminal-too-small.md](terminal-too-small.md).

## Recompute triggers

The list width is a pure function of live parameters, so every
trigger simply flows through the existing keyed-layout path:

- **File loading** can grow the gutter (a five-digit line number
  widens it), which both shrinks the third formula term and widens
  the panel reservation — the layout key is minted per buffer with
  that buffer's own gutter, so a key minted for any path is
  self-consistent.
- **Mode changes** (`w` toggling wrap) change the reserved
  indicator width.
- **Terminal resizes** change both the cap and the reservation.
- **Hide/show** changes the text width the layout must be built
  for.

Every one requests a `requestLayout` for the current file; a
completion installs only while its key still matches the live
`layoutKey`, and `currentRows` hides a stale installed model so the
panel shows the placeholder until the replacement arrives. The
saved viewport's logical `(line, column)` anchor — never a
rendered-row ordinal — restores into the fresh model, so the
reading position survives hide/show, gutter growth, mode changes,
and resizes alike.

## Visibility preference versus computed width

`m.listVisible` is the user's preference, initialized `true`;
`listWidthFor` returns 0 while it is false. The two are
independent: a computed width of zero draws no list cells but does
**not** clear the preference — no automatic visibility toggle ever
occurs, so widening the terminal back restores the list without a
keypress.

`left` and `tab` hide; `right` and `shift+tab` show. Both re-key
the current file's layout through `requestLayout` so the panel
text-width change takes effect with anchor preservation. Pressing a
hide or show key that is already in effect is a no-op.

## Path truncation

`listCell` escapes only the requested visible entry, left-truncates
it with `leftTruncate` — a leading `…` plus the longest whole-
grapheme tail that fits, so the basename end stays visible — applies
the file-list or current-file (underlined) style, and pads to the
allotted cells. Clusters are never split.

## Auto-scroll

The list shares the `h−1` rows below the filename rule with the
file panel and scrolls to keep the cursor-derived current entry
visible: once `curFile` reaches index `h−1` the window top becomes
`cur − h + 2`, pinning the active entry to the last row as `n`/`p`
cross files. The list remains a passive overview — scrolling
follows the matched-line cursor, never the other way.

## The filename-row status-note slot

`filenameRule` composes `─ path note ────` across the full frame:
the buffer-status note sits in a slot after the path, and the path
left-truncates with a leading `…` to make room for the note where
possible. When the note itself would overflow, the path yields its
cells first and the note clips to whatever the slot leaves —
nothing overflows the frame width.

The slot is real and Issue #29 is its only supplier so far:
`m.notes[path]` carries the stale/file-changed note while the
buffer's mark holds. Issue #26's "(unreadable)" and Issue #30's
"(unsupported encoding)" are panel placeholders, not notes — this
issue provides the composition seam the note writes into.

## Tests

`internal/app/filelist_test.go` (same package) drives every
contract:

- `TestFileListWidthFormula` — each of the three terms winning in
  turn, `floor(0.40 × w)` rounding at an odd width, the ten-cell
  text reservation outranking the 40% cap, gutter growth narrowing
  the list, and the zero clamp.
- `TestListHideShowToggles` — `left`/`tab` hiding and
  `right`/`shift+tab` showing, each issuing a re-keyed layout whose
  install reclaims the panel width.
- `TestListToggleIdempotent` — a repeated hide or show key changes
  nothing.
- `TestZeroWidthListRetainsPreference` — a computed zero width
  leaves the preference set and the list returns when the terminal
  widens.
- `TestLeftTruncateGraphemeSafe` / `TestListEntriesLeftTruncate` —
  the `…` tail truncation never splitting a grapheme, in the helper
  and in rendered list cells.
- `TestFilenameRuleStatusSlot` / `TestFilenameRuleNoNote` — the
  synthetic note inside the rule with the path truncated around it,
  the note clipping under a tiny width, and the note-free rule
  unchanged.
- `TestListAutoScrollsToActiveEntry` — `n` crossings scrolling the
  window so the active entry stays visible.
- `TestAnchorSurvivesListToggle` /
  `TestAnchorSurvivesGutterGrowth` — the same logical line at the
  top of the panel after a hide/show round trip and after a gutter-
  widening file load.
- `TestHiddenListEscapesNoEntries` — the render-cost guard: a
  hidden list escapes zero entries, and a visible one escapes only
  the visible window's.

The `dropCells` cell-aware slicing helper joins `clipCells` for
tests, since `…`'s three bytes occupy one cell. Existing
width-sensitive tests moved to `m.listWidth()` and `frameWidthForG`
(gutter-explicit frame sizing under the new formula), and the
panning/indicator/grapheme/wrap suites slice panel rows by cells
rather than bytes.
