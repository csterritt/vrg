# Overlay full scroll — no head/tail compression (Issue #41)

Issue #41 removes the head-plus-ellipsis-plus-tail compression that
`renderOverlay` applied to non-help diagnostics longer than the
visible height. The omitted middle rows could never be reached, which
made long captured stderr streams — especially large ripgrep stderr —
partially inaccessible. Every wrapped row of the diagnostic now stays
in the scrollable set. Relevant PRD sections: *Colours, overlays, and
key precedence*, *Outcome and exit-status contract*. See also
[outcome-contract](outcome-contract.md),
[overlay-precedence](overlay-precedence.md),
[help-overlay](help-overlay.md), and
[stream-integrity-diagnostics](stream-integrity-diagnostics.md).

## Complete scrollable row set

- The scrollable row set of a non-help overlay is the complete wrapped
  diagnostic: every wrapped row is retained and reachable.
- No ellipsis row is injected and no middle rows are elided at the
  model level; head/tail summarization is forbidden.
- Render-time clipping to the visible window (and to the terminal at
  tiny sizes, Issue #31) remains allowed — clipping affects only what
  is painted, never the row set.
- Appended diagnostics extend the scrollable set while preserving the
  reader's current position (the Issue #32 append-preserving-scroll
  primitive).
- Help-overlay scrolling is unchanged: the complete-row rule now holds
  uniformly for every overlay kind.

## Scroll clamp

`overlayScroll` is clamped to `[0, max(0, rows−maxVisible)]`, where
`rows` is the complete wrapped row count and `maxVisible` is the
visible window (terminal height minus border and margin):

- In `handleOverlayKey`: `up`/`down` move one row within the clamp.
  A stale position beyond the bound (for example after a resize grows
  the window) snaps back into range on the next scroll key.
- In `renderOverlay`: the visible slice is clamped against the
  complete row set, so a stale position renders the last page rather
  than running off the end.

The modal key contract is unchanged: `up`/`down` scroll, `q`/`Esc`
dismiss, `ctrl+c` exits 130, and every other key — including the
base-state scroll bindings `u`, `d`, page up, and page down — is
ignored while an error overlay is open.

## Implementation

`overlayInteriorWidth` and `overlayMaxVisible` compute the wrap width
and window height shared by the key handler and the render path.
`overlayRows` returns the complete wrapped row set and memoizes it in
`overlayWrapCache`, keyed on the exact (text, interior-width) inputs,
so the per-keypress clamp does not re-wrap a large diagnostic (about
166 ms for a ~1 MiB overlay text on one reference machine). Appends
and resizes invalidate the entry naturally because the key inputs
change. `overlayMaxScroll` derives the clamp bound from the cached
rows.

## Large-stderr fixture contract

Issue #41 supersedes the Issue #9 requirement that the ≥1 MiB stderr
fixture render its head and tail simultaneously. The PTY fixture
(`TestStderrContentFixture`) still proves stream drainage, a complete
stdout stream, captured stderr in diagnostics, and completion, but it
no longer requires head and tail in one frame — the head is visible
when the overlay opens and the tail is a real row at the end of the
complete set. Tail reachability is proven by the model-level
complete-rows and bounded-traversal tests, including a ≥1 MiB model
fixture, rather than by sending thousands of PTY keys.

The fixture scrolls a few rows and then dismisses with `q` twice
(first `q` closes the overlay, second exits). A bare `Esc` followed by
`q` is avoided there: while an expensive update is in flight the
input reader can coalesce `Esc`+`q` into a single `Alt+q` press, which
the modal overlay ignores.

## Cross-references

- [Issue #41](../issues/041-overlay-full-scroll-no-head-tail-compression.md)
- [PRD: Colours, overlays, and key precedence](../PRD-vrg.md)
- [Outcome contract (Issue #9)](outcome-contract.md) — origin of the
  superseded simultaneous head/tail requirement
- [Overlay precedence (Issue #32)](overlay-precedence.md)
- [Help overlay (Issue #31)](help-overlay.md)
- [Stream-integrity diagnostics](stream-integrity-diagnostics.md)
