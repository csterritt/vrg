# Terminal too-small screen with full state recovery

Issue #33 — the "Terminal too small" screen shown when the terminal
drops below the 20×3 minimum, with full state preservation and
recovery on resize.

Cross-references: [overlay-precedence](overlay-precedence.md) (Issue
#32 dismissal semantics, which the too-small `q` rule takes
precedence over), [file-list-layout](file-list-layout.md) (Issue #24
layout), [help-overlay](help-overlay.md) (Issue #31 help overlay
state preserved during too-small), [outcome-contract](outcome-contract.md)
(Issue #9 exit-status contract), `Notes/PRD-vrg.md` (Layout and
indicators — minimum-size bullet).

## Threshold and display

The fixed terminal minimum is **20 columns and 3 rows**. When either
dimension falls below this minimum, the `tooSmall` flag is set by the
`WindowSizeMsg` handler and the `View` renders the centred
**"Terminal too small"** message (via `centerText`) instead of the
normal browse, no-results, or searching screen. The message is centred
"as space permits" — at pathological sizes (e.g. 10×1) the text may
not fit fully, but the too-small gate is still installed.

The `TooSmall()` accessor reports whether the gate is active. The
`Theme()` accessor returns the active colour scheme (preserved across
too-small round trips).

## Active keys

Only **`q`** and **`ctrl+c`** are active on the too-small screen:

- **`ctrl+c`** exits 130 (handled before the too-small check, same as
  every other state).
- **`q`** exits with the **state-applicable outcome**, taking
  precedence over Issue #32's dismissal semantics. Even if a modal
  overlay is logically open, `q` exits the program rather than
  dismissing the overlay:
  - Searching → exit 130 (cancellation via `cancel()`).
  - Browse → exit with the fixed search-derived status (0 for a clean
    search, 2 for a fatal search with results).
  - No-results → exit 1.
  - Fatal no-results overlay logically open → exit 2.
  - Browse error overlay logically open → exit 2 (the fixed status),
    not merely dismissing the overlay.
- **`Esc`** and every other key are **no-ops**. A logically open
  overlay remains open; the program does not exit.

The `quitTooSmall()` helper implements the `q` path: it calls
`cancel()` for the searching state (exit 130), sets `exitCode = 0` for
the summary state, and otherwise quits with the fixed exit status
already decided at completion.

## Preserved state

The too-small gate preserves **all** state for recovery on resize —
no layout is installed, no anchors are mutated, and no partial modal
restoration occurs while the gate is installed:

- **Cursor selection** — the matched-line cursor position and current
  file.
- **Per-file viewport state and logical anchors** — the vertical
  offset and the width-independent logical reading anchor for each
  file.
- **List visibility preference** — the `left`/`tab` hide and
  `right`/`shift+tab` show preference.
- **Wrap and colour settings** — the wrap mode and the colour scheme.
- **Horizontal offset** — the pan offset in run-off-edge mode.
- **Full modal state**:
  - Which overlay is open (help, error, warning).
  - The help overlay scroll position.
  - The error overlay scroll position.
  - Any help-suspended-by-error relationship (the `suspendedHelp` flag
    and `suspendedHelpScroll` position from Issue #32).
- **Pop-up timer** — an active file-change pop-up's timer continues
  during too-small. The pop-up is **not displayed** on the too-small
  screen. An expiry arriving during too-small dismisses the pop-up so
  it is absent after recovery.

## Recovery

When the terminal grows back to at least 20×3, the `WindowSizeMsg`
handler clears the `tooSmall` flag and rebuilds the viewport from the
final dimensions via the normal `buildViewport` path. Because no
state was mutated during too-small, the existing anchor-preservation
and per-file-offset-restore logic recovers the full prior state:
cursor, viewport, anchors, list visibility, wrap, colour, horizontal
offset, and the complete modal stack with scroll positions.

## Resizes wholly within the too-small state

A sequence of resizes that stay below 20×3 (e.g. 19×2 → 10×1 → 25×8)
keeps the gate installed at every sub-minimum step. No ordinary layout
is installed at the pathological dimensions, no anchors are mutated,
and no partial modal restoration occurs. Recovery uses the **final**
dimensions (25×8 in the example), not any intermediate size. A
scrolled help overlay, an error-over-help stack, and a nontrivial
viewport state are all preserved without loss across the full
sequence.

## Precedence over Issue #32 dismissal semantics

The too-small `q` rule takes precedence over Issue #32's overlay
dismissal semantics. In normal operation, `q` dismisses a non-fatal
overlay (returning to the base state) or exits 2 for a fatal
no-results overlay. On the too-small screen, `q` **always exits** —
it does not dismiss an overlay, even if one is logically open. This
ensures the too-small screen never traps the user behind an invisible
modal overlay. `Esc` remains a no-op on the too-small screen and does
not dismiss the hidden overlay; the overlay is still open after
recovery.
