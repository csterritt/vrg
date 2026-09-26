# "Terminal too small" screen with full state recovery (Issue #33)

The minimum-size contract delivered by
[Issue #33](../issues/033-terminal-too-small-with-state-recovery.md),
implemented in `internal/app` (`toosmall.go` plus gates in `app.go`'s
`Update`/`View` and `browse.go`'s `syncLayout`). Relevant PRD section
in `Notes/PRD-vrg.md`: *Layout and indicators* — the fixed terminal
minimum bullet.

## The threshold

The fixed terminal minimum is **20 columns and 3 rows**
(`minTermCols`/`minTermRows`). `m.tooSmall()` reports true below
either dimension; exactly 20×3 is viable. While the predicate holds,
`View` renders only the gate screen — `renderTooSmall` paints
"Terminal too small" centred on the middle row, horizontally padded
and clipped to the frame when the text itself is too wide — with no
underlying screen, pop-up, or overlay composited. The gate therefore
owns the display in every phase, including `phaseSearching`.

## Active keys and exit semantics

`Update`'s `KeyPressMsg` branch routes to `tooSmallKey` ahead of the
overlay and help handlers — the gate owns the keyboard above the whole
[precedence stack](overlay-precedence.md):

- **`q`** exits with the state-applicable outcome: the cancellation
  path (`ExitCode` 130, child termination signalled) while searching;
  otherwise the settled fixed status — the search-derived status in
  browse (0 or 2), 1 on no-results, 2 when the fatal no-results
  overlay is logically open.
- **`ctrl+c`** exits 130 from any state, as always.
- **`Esc` and every other key are no-ops.** A logically open overlay
  keeps its scroll position and stays open for recovery.

The decisive rule: too-small `q` **exits rather than dismissing** a
logically open overlay — this takes precedence over Issue #32's
dismissal semantics, so the invisible modal can never trap the user.
`Esc` remains a no-op and does not dismiss the hidden overlay. Inert
keys are complete no-ops under the gate — the gate sits ahead of the
usual any-key pop-up dismissal in `Update`, so a stray `Esc` or `n`
cannot silently cancel a pop-up the user cannot see.

## Full state preservation and recovery

The gate does not snapshot — it freezes. `syncLayout` early-returns
while `tooSmall()` holds, so a resize into or within the gate records
the new dimensions but does no layout work: no `Viewport.Resize`
(which would re-resolve the top and could lose the logical anchor to
the lossy clamp), no list re-scroll, no layout request minted at
pathological widths. Everything underneath survives a round trip
unchanged:

- cursor selection and per-file saved viewport state (`saved`
  anchors),
- the current logical anchor and horizontal pan offset,
- the file-list visibility preference, wrap mode, and colour scheme,
- full modal state — which overlay is open, the help and error scroll
  positions, and the help-suspended-by-error relationship.

Because nothing is rebuilt at pathological sizes, a resize wholly
within the gate (19×2 → 10×1) mints no layout request and moves no
anchor; the resize that returns to a viable size runs `syncLayout`
against the **final** dimensions, issuing the stale-keyed layout
request that reinstalls content and commits any pending reveal intent.

Work continues underneath the gate: a `searchDoneMsg` still resolves
its outcome (phase, fixed status, a logically opened overlay) and a
`loadDoneMsg` still stores its buffer — both deferring layout to
recovery.

## Pop-up timer continuation

An active file-change pop-up keeps its instance and its one-second
timer while the gate is up — the resize issues no timer command and
the pop-up is simply not painted. A `popupExpiredMsg` arriving during
too-small dismisses the instance normally, so it is absent after
recovery; an unexpired instance reappears on the recovered frame. See
[file-change-popup.md](file-change-popup.md).

## Tests

`internal/app/toosmall_test.go` pins the contract through `Update`
alone:

- `TestTooSmallThreshold` — the 20×3 boundary in both directions.
- `TestTooSmallMessageCentredAsSpacePermits` — centring at 19×10 and
  40×2, clipping at 10×1.
- `TestTooSmallQExitByState` — `q` exits 0 browsing, 2 under a fatal
  search, 1 on no-results, 2 with the fatal overlay logically open,
  and exits rather than dismissing with a browse error overlay, help,
  or an error-over-help stack logically open.
- `TestTooSmallQWhileSearchingCancels` — `q` exits 130 with the child
  signalled.
- `TestTooSmallCtrlCExits130` — `ctrl+c` exits 130 in every state.
- `TestTooSmallKeysAreNoOps` — `Esc` and every other key produce no
  command and no state change under an error-over-help stack, which is
  still open at its scroll positions after recovery.
- `TestTooSmallRoundTripRestoresBrowseState` — cursor, per-file saved
  anchors, current anchor and top, horizontal offset, list visibility,
  wrap, colour, and cached geometry across a shrink-and-grow.
- `TestTooSmallRoundTripRestoresModalState` — scrolled help, a
  scrolled error overlay, and an error-over-help stack at their prior
  positions, with the restored stack still behaving.
- `TestTooSmallInteriorResizeDefersRecovery` — the 19×2 → 10×1 → 25×8
  chain: no command, no minted layout key, no anchor/saved/modal
  movement at 10×1, and recovery keyed to the final 25×8 dimensions.
- `TestTooSmallCompletionBehindGate` — a search completing under the
  gate resolves browse, stores the load, and presents on recovery.
- `TestTooSmallPopupTimerContinues` /
  `TestTooSmallPopupHiddenNotCancelled` — the timer is not restarted,
  the pop-up never paints on the gate, inert keys under the gate do
  not dismiss the hidden instance, an expiry during too-small
  dismisses it for good, and a live instance reappears after
  recovery.

## Files

- `internal/app/toosmall.go` — `minTermCols`/`minTermRows`,
  `tooSmall`, `tooSmallKey` (the `q`/`ctrl+c`-only keyboard), and
  `renderTooSmall` (the centred, clipped message).
- `internal/app/app.go` — the `KeyPressMsg` gate ahead of the modal
  routers and the `View` gate ahead of all compositing.
- `internal/app/browse.go` — the `syncLayout` early return that
  freezes geometry and defers recovery to the final dimensions.
- `internal/app/toosmall_test.go` — the contract tests.

See also:
[overlay-precedence.md](overlay-precedence.md) (the stack the gate
preempts and the `q`-exits rule's precedence over it),
[help-overlay.md](help-overlay.md) and
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
(the modals whose scroll state survives),
[file-change-popup.md](file-change-popup.md) (the continuing timer),
[logical-anchor-and-layout.md](logical-anchor-and-layout.md) (the
anchors and keyed layouts the freeze protects), and
[no-results-and-binary-exclusion.md](no-results-and-binary-exclusion.md)
(the sibling centred-message screen).
