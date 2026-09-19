# Bounded browse render — path metadata prepared once, per-frame work bounded by the visible window

Issue #40 lands PRD *Resources and responsiveness*'s final browse-side
bound. Issue #24 had already fixed `listWBase` and hidden-list
skipping; Issue #39 supplied the shared grapheme/cell model. What
remained was per-frame path work: `listCell` re-escaped and
re-segmented each visible row, and `filenameRule`/`compositePopup`
re-escaped the current path every frame. At ~100,000 matched-line
scale every `n` keypress then re-sanitised and re-segmented the same
immutable paths. Search completion now prepares that path display
metadata once — navigation `Update()` and `View()` escape and
segment nothing.

## Prepared once at `searchDoneMsg`

`model.displayPaths []displayPath` is allocated alongside the search
index in the `searchDoneMsg` handler, one entry per
`searchindex.Index.Files[i]`:

```go
escaped := m.escapePath(m.idx.Files[i].Path)
m.displayPaths[i] = newDisplayPath(escaped)
```

A `displayPath` carries everything width-independent:

- `text` — the `EscapePath`-sanitized single-line path (Issue #11's
  sanitisation contract, now invoked once per file per index);
- `width` — `safepresentation.CellWidth(text)`;
- `clusters` — a `[]pathCluster` of byte-offset/cell-width pairs
  marking every grapheme boundary, so later truncation never
  re-segments.

`m.listWBase` — the longest prepared `width` plus one — is computed in
the same loop, still feeding Issue #24's `listWidthFor` formula and
Issue #38's `panelW`/`frameWidthFor` shrink paths. It costs no extra
pass over the index.

The grouping itself is `m.idx.Files` — a
`searchindex.Index`-owned slice the model treats as immutable for the
index's lifetime — plus cursor state (`navIndex`). No copy or
regrouping happens on navigation or render.

## Per frame

`listCell` calls `m.entry(i)` — a bounds-checked accessor returning
`displayPath{text, width, clusters}` — and left-truncates **only the
requested row** via `displayPath.leftTruncate`. The truncation walks
`clusters` from the back until `…` plus that many cells fits the
current list width — never splitting a grapheme, never re-segmenting.
`filenameRule` and `compositePopup` consume
`m.entry(m.curFile())` the same way: the current path is escaped once
at index time, and only its width-dependent clip is per-frame. The
generic `leftTruncate` helper survives for non-path callers
(`[no stop]`-style literals) and delegates to `newDisplayPath`.

Because the truncated text is the only width-dependent quantity, the
things that change the list width — terminal resize, gutter growth,
wrap-mode indicator toggles, list hide/show — re-truncate the visible
window against the same prepared metadata; nothing rescans the index
or re-escapes paths.

Navigation (`n`/`p`/`w`, list toggles) mutates only cursor and
viewport state; `TestNavigationKeepsPreparedGroups` pins both
`&m.idx.Files[0]` and `&m.displayPaths[0]` pointer-stable across a
1000-file index's `n`/`p` round trips.

## Cost guard

`layout_test.go`'s counting `escapePath` seam spans a navigation
`Update()` **and** the resulting `View()` without resetting the
counter:

- `TestNavigateAndRenderEscapeNoPaths` — a post-search `n` press plus
  its frame escapes zero paths.
- `TestRenderEscapesNoPaths` — a bare `View()` over a 50-file index
  escapes zero paths (renamed and strengthened from #24's
  `TestRenderEscapesOnlyVisibleListEntries`).
- `TestResizeRetruncatesFromPreparedPaths` — a resize + hidden-list +
  resize sequence produces correctly re-truncated frames and escapes
  zero paths.
- `TestHiddenListEscapesNoEntries` (#24, updated expectation) — the
  visible-window loop escapes zero entries.
- `TestNavigationKeepsPreparedGroups` — navigation reallocates
  neither the file grouping nor the prepared path metadata.

The bound is on per-keypress/per-frame work, not preparation:
`searchDoneMsg`'s one-time pass is O(index) and amortised across every
frame the index serves.

## Sources

`internal/app/app.go` (`displayPaths` field, `searchDoneMsg`
preparation, `escapePath` seam comment),
`internal/app/browse.go` (`displayPath`, `pathCluster`,
`newDisplayPath`, `entry`, `listCell`, `filenameRule`),
`internal/app/popup.go` (`compositePopup`, `leftTruncate`),
`internal/app/layout_test.go`, `internal/app/filelist_test.go`,
`Notes/issues/040-browse-render-no-whole-index-scan.md`,
`Notes/tasks/040-browse-render-no-whole-index-scan.md`, PRD *Resources
and responsiveness* and *File list and layout*.
