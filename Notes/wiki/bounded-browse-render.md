# Bounded browse rendering from prepared file groups (Issue #40)

Issue #40 removes the per-frame whole-search-index scan from the
browse render path. Before this issue, every `View()` called
`groupByFile(m.index.Stops())` — copying the full stop slice and
regrouping all files — `index.Files()` rebuilt a distinct-path set
over every stop, the current-file lookup scanned the regrouped slice
linearly, and each visible entry re-escaped and re-segmented its path.
At the PRD's ~100,000-matched-lines scale (see
`Notes/PRD-vrg.md` — *Resources and responsiveness*), a single
keystroke's `Update()` + `View()` allocated tens of megabytes, so
navigation and repainting stalled proportionally to result size.

Depends on [Issue #39](../issues/039-render-from-shared-grapheme-cell-model.md)
(the shared grapheme/cell model and the width-independent cluster
table). This page cross-references:

- [file-list-layout](file-list-layout.md) — the list width formula,
  truncation, and auto-scroll whose per-frame cost is now bounded.
- [shared-cell-model-render](shared-cell-model-render.md) — the
  helper the precomputed metadata consumes.
- [logical-anchor-and-layout-preparation](logical-anchor-and-layout-preparation.md)
  — the earlier render-cost guards for visible rows and file-list
  entries (Issue #17).
- [manual-vertical-scrolling](manual-vertical-scrolling.md) — the
  visible-range-only viewport render guard.
- `Notes/issues/040-browse-render-no-whole-index-scan.md` — the issue.
- `Notes/PRD-vrg.md` — the *Resources and responsiveness* and
  *File list and layout* sections.

## Prepared once at search completion

The `SearchCompleteMsg` handler calls `prepareFileGroups(idx)`, which
performs a single pass over the finalized index and produces:

- `m.fileGroups []fileGroup` — the immutable per-file grouping of the
  index's stops. Stops arrive ordered by unsigned raw path bytes then
  line number, so a file's stops are contiguous and each group holds
  its own `stops` subslice. The groups are never regrouped or
  reallocated afterwards.
- `m.fileIndexByPath map[string]int` — raw path → group index, so the
  current-file lookup (`currentFileIndex`) and the file loader's
  per-file stop range (`loadFileFor`) are map accesses instead of
  whole-index scans.
- Width-independent path metadata stored on each `fileGroup`:
  `escaped` (the `safepresentation.EscapePath` text), `clusters` (the
  `safepresentation.GraphemeClusters` table — the Issue #39 shared
  policy), and `width` (the full cell width, summed from the cluster
  table).
- `longestPathWidth` — the longest escaped path's cell width, term 1
  of the Issue #24 list width formula, falls out of the same pass
  (replacing `computeLongestPathWidth`).

## What remains per frame

`renderBrowse` reads `m.fileGroups` directly and iterates only the
visible `[listOffset, listOffset+visibleRows)` window — the provider
and precomputed-group paths are both bounded by the terminal height.
Per visible row, `truncateFileEntry(group, listWidth)` left-truncates
the stored escaped text through
`safepresentation.TruncateLeftCellsFrom` — the new cluster-table
variant of `TruncateLeftCells`, so no per-frame escaping or
re-segmentation occurs. The truncated strings are deliberately *not*
precomputed: the allotted list width changes on terminal resize,
line-number gutter growth, wrap-mode indicator changes, and list
hide/show, so truncation runs inside `View()` against the current
width — still bounded by the visible row count, and still grapheme-safe
because the stored cluster table is the same shared policy.

The content panel's filename row uses the group's stored `escaped`
text, and the `len(m.fileGroups) == 0` check replaces the per-frame
`index.Files()` distinct-path scan.

Navigation (`handleNavigate`) updates the cursor and calls
`updateListOffset(m.currentFileIndex(stop.RawPath), …)` — a map lookup
plus window arithmetic; it neither regroups nor allocates per stop.

## Strengthened cost guard

`internal/app/layout_test.go` adds a combined `Update()` + `View()`
guard: `measureUpdateViewAllocs` snapshots `runtime.MemStats` (after a
`runtime.GC()`) across one navigation or resize transition *and* the
resulting `View()` without resetting between them, requiring
≤ 512 KiB / ≤ 4096 mallocs against a synthetic 3,000-file / 45,000-stop
index. Because the counter spans both halves, moving the whole-index
work into `Update()` — navigation handling, message processing,
anywhere in the transition — fails the bound just as a `View()` scan
does. The existing `countingFileList` provider assertions pin the
visible-window contract alongside.

## Responsiveness rationale

At ~100,000 matched lines the pre-fix frame copied and regrouped the
entire stop slice several times per keystroke (~40 MB allocated per
navigation in the guard test). After Issue #40 a transition touches
only the visible window — tens of rows — so per-keystroke cost is
independent of index size and repainting stays responsive under held
keys and repeated resizes.

## Tests

See [unit-tests](unit-tests.md) for the Issue #40 catalog entries in
`internal/app/layout_test.go`: `TestRenderCostGuardNavigateViewBounded`,
`TestRenderCostGuardNavigatePrevBounded`,
`TestRenderCostGuardResizeRetruncates`, and
`TestRenderCostGuardGutterGrowthRetruncates`.
