# Full-scroll error overlay — no head/tail compression (Issue #41)

Delivered by
[Issue #41](../issues/041-overlay-full-scroll-no-head-tail-compression.md)
([task](../tasks/041-overlay-full-scroll-no-head-tail-compression.md)):
the error overlay's scrollable row set is the **complete** wrapped
diagnostic — no head-plus-ellipsis-plus-tail compression ever removes
rows from the model — and every row of an arbitrarily long diagnostic
is reachable by `up`/`down`. Relevant PRD section: *Colours, overlays,
and key precedence* ("vertical scrolling reaches all rows at usable
sizes") in [`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 80–83.
Builds on
[error-overlay-and-outcomes.md](error-overlay-and-outcomes.md) (the
Issue #9 overlay), [help-overlay.md](help-overlay.md) (the shared
`scrollBox`/`compositeBox` component), and
[overlay-precedence.md](overlay-precedence.md) (the modal stack).

## The complete-row contract

- **Nothing is elided from the model.** `scrollBox.rows` returns every
  wrapped row of the diagnostic — head, middle, and tail alike — and
  no ellipsis marker is ever injected as a substitute for content. If
  head/tail summarisation is ever wanted it must be an explicit
  alternate view, which this issue does not require.
- **One clamp, both paths.** The scroll position is clamped to
  `[0, max(0, rows − visible)]` where the key handler moves it
  (`scrollBox.scrollBy`/`clamp` via `scrollOverlay`) **and** where the
  render path derives the visible window (`compositeBox` clamps its
  local scroll index before slicing). `clampOverlayScroll` re-applies
  the same bound on every non-gated resize, so growth past the
  diagnostic returns the reader to the first row.
- **Every row is reachable.** A bounded row-by-row `down` traversal
  walks the visible window across the whole set to the final line and
  an `up` traversal walks it back to the first — each step moves the
  window exactly one row. This holds for the ≥ 1 MiB stderr shape
  without sending thousands of keys: the tail is proven reachable by
  the complete-set membership plus the clamp bound, not by one frame
  showing both ends.
- **Appends extend the set.** `openOverlay` appends a new diagnostic
  to the open overlay without moving the reader's scroll position;
  the appended text is reachable at the bottom — the Issue #26
  append-preserving-scroll primitive, unchanged.
- **Clipping is a render concern only.** At tiny sizes above the
  [20×3 minimum](terminal-too-small.md) the box still clips to the
  terminal at render time (story 83) — clipping what a frame shows is
  fine; removing rows from the scrollable set is not. Growth restores
  the normal layout.
- **The modal key contract is unchanged.** `up`/`down` scroll,
  `q`/`Esc` dismiss, `ctrl+c` exits 130, and every other key —
  including the base file-content bindings `u`, `d`, `pgup`, and
  `pgdn` — is ignored while the overlay is open. The help dialog's
  scrolling over the same `scrollBox` is unchanged.

## Superseded fixture semantics

This contract **supersedes Issue #9's original requirement** that the
≥ 1 MiB stderr fixture render head and tail simultaneously in one
frame. `cmd/vrg/outcome_test.go`'s `TestStderrContentFixture` keeps
its pipe-drainage, complete-stdout, captured-stderr inclusion, and
completion assertions but no longer demands simultaneous head/tail
visibility — tail reachability is proved by the model-level
complete-rows, clamp, and bounded-traversal tests instead.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/app/overlay_test.go` —
`TestOverlayKeepsCompleteDiagnostic` (the ≥ 1 MiB set contains both
markers with no ellipsis row, and the clamped scroll reaches the
tail), `TestOverlayBoundedTraversalReachesEveryRow` (the barely
oversized fixture's row-by-row walk to the final line and back),
`TestOverlayRenderClampsScroll` (the render path clamps out-of-range
scroll and resize reclamps the position), and
`TestOverlayKeyRoutingAndScrolling` (the unchanged ignored-key sweep,
`u`/`d`/page keys included); `precedence_test.go`'s
`TestAppendedErrorPreservesReaderPosition` and
`TestErrorOverHelpRoutesKeysToError` cover appends over browse and
over suspended help; `cmd/vrg/outcome_test.go`'s
`TestStderrContentFixture` keeps the revised ≥ 1 MiB semantics.
