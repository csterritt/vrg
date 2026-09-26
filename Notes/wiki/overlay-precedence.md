# Overlay precedence and `Esc`/`q` dismissal semantics (Issue #32)

The combined precedence and dismissal contract delivered by
[Issue #32](../issues/032-overlay-precedence-esc-semantics.md),
implemented in `internal/app` (`app.go`'s `KeyPressMsg` routing plus
`overlay.go`/`help.go`/`popup.go`). Relevant PRD sections in
`Notes/PRD-vrg.md`: *Colours, overlays, and key precedence* (the
precedence stack, the `Esc` bullet, the error/help bullets, the
suspension line) and *Outcome and exit-status contract* (the outcome
table and the `q`/`Esc` dismissal bullet). Issue #33 owns the
too-small screen's dedicated rule, which takes precedence over this
stack.

## The key-precedence stack

`Update`'s `tea.KeyPressMsg` branch implements the ordinary
browse/no-results precedence in order:

1. **`ctrl+c`** — global; every tier's handler takes the cancellation
   path to exit 130.
2. **Modal error** — `m.overlay != nil` routes the key to
   `overlayKey` before anything else.
3. **Help** — `m.help != nil` routes to `helpKey` next.
4. **Pop-up** — any key press clears `popupID` before routing, so a
   key both dismisses the pop-up and performs its normal action in the
   same update; `Esc`'s dismissal is its only effect.
5. **Base-state keys** — the underlying screen's own bindings.

The order means an open error overlay routes every key — including
`h`/`?`, `n`, `w`, `c`, `r`, and the scrolling keys — to itself:
`up`/`down` scroll the error's rows while suspended help's offset does
not move, and help's close keys are swallowed.

## Error suspends help

A new error arriving while help is open suspends help rather than
destroying it: `openOverlay` never touches `m.help`, so the help
instance keeps its lines and scroll offset, `m.overlay` takes the
keyboard and `View`'s topmost composite, and dismissing the error
with `q` **or** `Esc` reveals help at its retained position. See
[help-overlay.md](help-overlay.md).

## Appended errors preserve the reader's scroll

`openOverlay` is the single opening route for every error overlay —
search-completion diagnostics, current-file read failures,
re-entry/reload prior-failure re-opens, and unsupported-encoding
explanations. When an overlay is already open it appends the new lines
and leaves `scroll` untouched: a reader sitting at position P stays at
P with the appended text reachable below, never jumping to the bottom.
Issue #26 introduced the primitive for reload re-entry failures; Issue
#32's `TestAppendedErrorPreservesReaderScroll` proves it holds for an
appended error from a different source (an unsupported-encoding
detection appended to a read-failure overlay). See
[read-failures.md](read-failures.md) and
[unsupported-encodings.md](unsupported-encodings.md).

## Pop-up cancellation without return

Opening help (`openHelp`) or an error (`openOverlay`) clears `popupID`:
a live file-change pop-up is cancelled and never returns when the
overlay closes, and the cancelled instance's late `popupExpiredMsg` is
inert because expiry is instance-keyed. See
[file-change-popup.md](file-change-popup.md).

## The `Esc`/`q` dismissal table

`Esc` is an overlay-dismissal key only — it never exits from a base
state. Both dismissal keys produce the same outcome in every row:

| State when `q`/`Esc` is pressed | Outcome |
|---|---|
| Browse + error overlay | Error closes; browsing resumes, still running |
| Browse + help | Help closes; browsing resumes, still running |
| Browse + error-over-help | Error closes; help restored at its scroll position, still running |
| Empty result + warning overlay | Overlay closes; the no-results screen shows, still running |
| No-results + help | Help closes; the no-results screen shows, still running |
| Fatal overlay, no usable results (fatal process/integrity failure) | **Exit 2** — no underlying state exists |
| Record-loss overlay, no usable results | **Exit 2** |
| Browse or no-results, no overlay | `q` exits with the fixed status (0/2 in browse, 1 in no-results); `Esc` is a no-op (pop-up dismissal aside) |

The state-specific follow-ups: from a still-running base state a
second `q` exits with the fixed status and a second `Esc` leaves it
running — but the error-over-help row asserts the three-key sequence:
the first dismissal closes the error and restores help, the second
closes the restored help to browsing, and only a further base-state
`q` exits. Dismissing the fatal no-results overlay with `Esc`
terminates with status 2 precisely because there is no underlying
state — the one place `Esc` exits, and never from a base state.

## Tests

`internal/app/precedence_test.go` pins the combined contract through
`Update` alone:

- `TestErrorSuspendsHelpRestoringScroll` — for both dismissal keys:
  the load gate parks the current file's failing worker, help opens at
  a reduced height and scrolls to row 5, the released failure opens
  the error over the suspended help, `down` scrolls the error while
  help's offset stays put and `h`/`n`/`r`/`w`/`c` are swallowed, and
  dismissal restores help at row 5.
- `TestAppendedErrorPreservesReaderScroll` — a read-failure overlay
  scrolled to 3 receives an unsupported-encoding append: the reader
  stays at 3 and the new text is reachable by scrolling to the bottom.
- `TestDismissalOutcomeTable` — every row of the dismissal table above
  run for both `q` and `Esc`, with the state-specific follow-ups
  (second `q` exits the fixed status, second `Esc` no-ops, and the
  error-over-help row's full three-key sequence).

Adjacent coverage: `TestEscOnBrowseIsNoOp`,
`TestEscOnNoResultsIsNoOp`, `TestEscDuringSearchingIsNoOp` (the
no-overlay `Esc` no-ops), `TestPopupDismissalKeys` (`Esc` over a
pop-up dismisses only), `TestHelpCancelsPopup` and
`TestErrorOverlayCancelsPopup` (cancellation without return), and
`overlay_test.go`'s dismissal-key and ignored-key tests. See
[unit-tests.md](unit-tests.md).

## Files

- `internal/app/app.go` — the `KeyPressMsg` precedence routing:
  pop-up dismissal, then `overlayKey`, then `helpKey`, then the
  base-state switch where `Esc` is unbound; `View`'s pop-up → help →
  diagnostics compositing order.
- `internal/app/overlay.go` — `openOverlay` (open-or-append preserving
  scroll, pop-up cancellation, help left suspended) and `overlayKey`
  (`q`/`Esc` dismissal, `phaseFatal` quitting outright).
- `internal/app/help.go` — `openHelp` (pop-up cancellation) and
  `helpKey` (`q`/`Esc`/`h`/`?` closing to the underlying state).
- `internal/app/popup.go` — the instance-keyed expiry that makes a
  cancelled pop-up's late timer inert.
- `internal/app/precedence_test.go` — the suspension, append, and
  dismissal-table tests.

See also:
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
(the outcome decision and the overlay's own key contract),
[help-overlay.md](help-overlay.md) (the suspended modal),
[file-change-popup.md](file-change-popup.md) (the cancelled pop-up),
[no-results-and-binary-exclusion.md](no-results-and-binary-exclusion.md)
(the `q`-exits-1 base state),
[cancellation-and-cleanup.md](cancellation-and-cleanup.md) (the
`ctrl+c` top tier).
