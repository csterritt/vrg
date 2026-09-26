# Logical anchor and off-UI layout preparation

Issue #17
(`Notes/issues/017-logical-anchor-through-rewrap-and-resize.md`, tasks
`Notes/tasks/017-logical-anchor-through-rewrap-and-resize.md`) gave the
reading position a width-independent form — the *logical anchor* — so
resize, rewrap, and wrap toggles preserve what the user is reading
rather than a row ordinal, and moved prepared-layout construction off
the Bubble Tea update path with keyed installation guards that isolate
obsolete results.

PRD cross-references: "Navigation, viewport, and logical anchors" (the
anchor bullet, the scroll/reveal replacement rules, and the
deliberately lossy EOF clamp) and "Resources and responsiveness" (the
responsiveness boundaries — expensive preparation off the update path,
inputs actionable while a worker runs, late results isolated) in
`Notes/PRD-vrg.md`.

## The logical anchor

`Viewport` keeps an `anchor Target` — a `(source line, display-column
offset)` location — alongside its effective `top` rendered row. The
anchor is the real reading position; the top is derived from it:

- `resolve()` recomputes the top as the rendered row *containing* the
  anchor under the installed row model — `Rows.RowOf(anchor)` — then
  clamps to valid content. `Resize` and `SetRows` both resolve, so a
  width change or a swapped row model lands the top on the row holding
  the anchor's text, not the row with the same former ordinal. Since
  Issue #18 every resolve and every scroll clamp also re-clamps the
  horizontal pan offset — a changed visible row set re-evaluates the
  extent maximum; see
  [horizontal-panning.md](horizontal-panning.md).
- `Row.Start` (Issue #17 addition) records the display-column offset
  where a rendered row begins, so a wrapped continuation row maps back
  to `(line, mid-line cell)` — `loc(i)` converts a row to its logical
  location. In run-off-edge mode every row has `Start` 0.
- `Anchor()`/`SetAnchor(Target)` expose the position in logical form;
  `SetTop` remains as the ordinal entry point and immediately replaces
  the anchor with the resulting top row's location.

Because the anchor is width-independent, wrap-off → wrap-on round
trips restore the logical column: a reader midway down a screen-tall
line returns to the same offset, not to the line's first row.

## Anchor replacement rules

The anchor follows whatever moved the viewport last:

- **Scrolling replaces it.** Any scroll that moves the top rewrites
  the anchor to the resulting top row's location (`loc(top)`). A
  scroll clamped to no movement — `up` at BOF — keeps it, so an
  ineffective keypress cannot discard a retained logical column.
- **A moving reveal replaces it; a no-scroll reveal keeps it.**
  `Reveal` already reports whether the top moved; only a move rewrites
  the anchor. A destination reveal that finds its target already
  visible therefore preserves a mid-line logical column the reader
  had.
- **EOF clamping rewrites it — deliberately lossy.** When `resolve`'s
  clamp pulls the top off the anchor's row (growth exposing avoidable
  blank rows below EOF), the anchor is rewritten to the clamped top's
  location. The pre-clamp position is intentionally unrecoverable: a
  later shrink need not restore it.

## Per-file saved state is the anchor

`Model.saved` maps raw path → `viewport.Target` (it was
`map[string]int` of top-row ordinals until Issue #17). Being
width-independent, a file's saved position survives rewraps and wrap
toggles that happen while it is not current. `navigate` writes the
departing file's `Anchor()` and `SetAnchor`s the destination's saved
value — the zero value, top of file, on a first visit — and `scroll`
records the post-scroll anchor. A resize preserves the matched-line
cursor selection automatically: the cursor lives in `Index`, and only
the derived top moves.

## Prepared layouts off the update path

`viewport.Prepare` — the row-mapping pass over the whole buffer — no
longer runs inside `Update`. Like file loads, it runs in a worker
`tea.Cmd` (`layoutCmd`) that delivers a `layoutDoneMsg{key, rows}`;
`Update` only installs the finished model.

- **Keys.** A layout request is minted with `layoutKey(path)` =
  `viewport.Key{Path, Rev, Width, Wrap}` — the raw path, the path's
  content revision (`revs`, bumped on every successful load), the
  current text width, and the wrap mode. `reqKey` records the latest
  requested key per path; `rows` caches each path's `installed{key,
  rows}` — the model with the key it was built for. Issue #38 pinned
  that the key's `Width` is always the layout-derived text width —
  panel width (terminal minus list width) minus the gutter and the
  reserved right-indicator column — computed once in `syncLayout`,
  never re-derived from the raw terminal width at an install site;
  see
  [file-list-layout.md](file-list-layout.md#terminal-panel-and-text-widths).
- **Install only on match.** A `layoutDoneMsg` installs only while
  its key equals the model's *current* `layoutKey` for that path.
  Out-of-order completions across successive resizes (W1→W2→W3) and
  rapid wrap toggles are superseded requests: they are discarded
  without touching the visible panel, the anchor, saved per-file
  state, or the pending reveal intent. A matching layout for a file
  that is no longer current still caches into `rows` — ready for a
  revisit — but never calls `SetRows`. Issue #27 exercises the
  remaining supersession source, the content revision: a layout keyed
  to a pre-`r` revision is discarded the same way (see
  [explicit-reload.md](explicit-reload.md)).
- **`syncLayout` recomputes the geometry** (list width, panel and
  text widths, viewport resize) after any parameter change — window
  resize, wrap
  toggle, search completion, a load changing the gutter, a file
  crossing, and since Issue #24 the file list's hide/show toggle —
  and returns `ensureLayout`'s request when the current
  file's installed layout is missing or stale-keyed. `ensureLayout`
  drops repeat requests for a key already in flight rather than
  queueing them. Issue #33 adds a guard ahead of all of this: while
  the window is too small `syncLayout` returns nil, no request is
  issued, and recovery replays it once against the first viable
  dimensions (see [terminal-too-small.md](terminal-too-small.md)).
- **Every input stays actionable while a worker is held.** Resize,
  `w`, `n`/`p`, `q`, and `ctrl+c` all complete their update without
  waiting for preparation; the layout tests hold the worker and drive
  each input to prove it.

## Pending reveal intent

A destination reveal needs rows to resolve against. When the current
file has no installed layout matching the current parameters —
uncached, or stale-keyed mid-rewrap — `reveal()` cannot run, so it
sets `pendingReveal` and returns. The intent is *newest-stop wins*:
`reveal` reads `currentStop()` at commit time, so navigation taken
while the worker is held reveals whichever stop is selected when a
matching layout installs — never a stale destination. `commitReveal`
runs the intent on each current-path install; `reveal` re-pends it if
the install still leaves no current rows. Issue #27 turned the boolean
into `pending pendingIntent` (`intentNone`/`intentReveal`/
`intentAnchor`) so an explicit reload can carry "keep the anchor, no
reveal" instead, and renamed the committer `commitIntent`; the
reveal intent's newest-stop semantics are unchanged. Issue #28 then
pinned the full two-stage contract on the seam: the load completion
performs no row-based decision (it computes the post-load gutter and
text width and requests the keyed layout), the intent survives every
obsolete install unmolested, and the commit — reveal for the newest
stop, or the anchor intent's silent clear — runs only against a
matching installed row model. See
[explicit-reload.md](explicit-reload.md) and
[load-completion-reveal.md](load-completion-reveal.md).

## Cached-file navigation

A file crossing recomputes the geometry first (the destination's
gutter may differ), then installs the destination's cached rows only
while their key matches — `currentRows()` returns nil for a
stale-keyed cache, leaving the viewport empty behind `Loading…` — and
applies the destination's saved anchor:

- **Stale or missing layout** → `ensureLayout` requests a prepared
  layout for the current parameters; the saved-anchor restore resolves
  when it installs, then the pending reveal commits over it.
- **Matching installed layout** → the fast path: `SetRows` installs it
  immediately, the saved anchor resolves synchronously, the reveal
  commits at once, and no request is issued.

## Render-cost guarantees

A frame touches only what it paints:

- `Visible()` queries the provider once per shown row — established
  with Issue #12, still guarded by the counting fakes.
- The row provider sees only lines behind visible rows (`Row`
  materializes per query; `TestRenderQueriesOnlyVisibleRows` installs
  a counting fake through a `layoutDoneMsg`).
- The file list gained an item-provider seam: `listEntry` renders one
  entry's label (production: `present.Path`; tests substitute a
  counting fake) and `renderBrowse` calls it only for the scrolled
  list window — `TestRenderQueriesOnlyVisibleListEntries` proves a
  200-file list renders one screen without an O(N) pass.
- `listW`/`textW`/`fileIdx` are computed on the update path inside
  `syncLayout`/search completion, so `View()` itself never rescans
  the file list.

## Tests

- `internal/viewport/anchor_test.go` — the anchor contract:
  width round trips and later-line round trips through rewrap,
  wrap-off→on column restoration, anchor replacement by scrolling and
  by moving reveals (including run-off-edge dropping a mid-line
  column), the no-scroll reveal's column retention, and the lossy
  EOF-clamp rewrite.
- `internal/app/anchor_test.go` — resize preserves the cursor
  selection and keeps the anchor's text at the top of the panel.
- `internal/app/layout_test.go` — the preparation contract: resize
  issues a keyed request off the update path; a held worker leaves
  `ctrl+c` (130), `q`, `n`/`p`, `w`, and further resizes actionable
  with the pending reveal preserved for the newest stop; out-of-order
  and superseded completions install nothing; a departed file's
  matching layout caches without touching the panel or saved state;
  an obsolete completion never consumes the pending intent;
  cached-file navigation requests a stale layout and commits intent
  on install while the matching-layout fast path commits immediately;
  and the list-entry render-cost guard.
- `internal/app/completion_test.go` — Issue #28's two-stage contract:
  the separately gated stages, no row decision at load completion,
  obsolete layouts never consuming the intent, resize and list-toggle
  supersession committing at the final width, startup and
  saved-viewport commits, marker and cluster-expanded targets, and
  non-current/pop-up isolation. See
  [load-completion-reveal.md](load-completion-reveal.md).

See [unit-tests.md](unit-tests.md) § `internal/viewport` and
`internal/app`.

## Files

- `internal/viewport/viewport.go` — `anchor`, `Anchor`/`SetAnchor`,
  `resolve`, `loc`, `Row.Start`, the replacement rules in `scroll` and
  `Reveal`, the lossy rewrite in `resolve`'s clamp.
- `internal/viewport/rows.go` — `rowSpan.start` feeding `Row.Start`.
- `internal/app/app.go` — `rows`/`revs`/`reqKey`/`saved` (now
  `viewport.Target`), `pending` (`pendingIntent` since Issue #27),
  `listW`/`textW`/`fileIdx`,
  `listEntry`, the `layoutDoneMsg` install guard, the `w` and
  `WindowSizeMsg` cases issuing layout requests.
- `internal/app/browse.go` — `layoutDoneMsg`, `installed`,
  `syncLayout`/`layoutKey`/`currentRows`/`ensureLayout`/`layoutCmd`,
  `reveal`'s pending intent, `commitIntent` (`commitReveal` until
  Issue #27), `navigate`'s crossing
  sequence, `renderBrowse`'s cached widths and visible-window list.

See also: [viewport-scrolling.md](viewport-scrolling.md) (the scroll
units and clamps built on this position),
[horizontal-panning.md](horizontal-panning.md) (the Issue #18 offset
that re-clamps on every resolve),
[destination-reveal.md](destination-reveal.md) (the reveal contract
the pending intent commits),
[wrap-mode.md](wrap-mode.md) (the keyed row model the guards consume),
[match-navigation.md](match-navigation.md) (the cursor steps the
intent follows),
[cancellation-and-cleanup.md](cancellation-and-cleanup.md) (the exit
paths that stay live while a worker is held),
[browse-tracer.md](browse-tracer.md) (the frame this work keeps
cheap), and [file-list-layout.md](file-list-layout.md) (the Issue #24
width changes — hide/show, gutter growth, mode flips — that route
through this relayout path).
