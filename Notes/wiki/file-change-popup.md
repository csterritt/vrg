# File-change pop-up — instance-keyed one-second notice (Issue #15)

Delivered by
[Issue #15](../issues/015-file-change-popup.md)
([task](../tasks/015-file-change-popup.md)): when `n`/`p` navigation
lands on a stop in a different file, a bordered box carrying the
destination file's path floats centred over the browse frame for one
second or until the next key press. Relevant PRD sections: *Colours,
overlays, and key precedence* (the pop-up bullet) and *File loading*
(the selection-time-start bullet) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 58, 59. Builds on
[match-navigation.md](match-navigation.md) (the `FileChanged` report
that triggers it), [theme.md](theme.md) (the shared `theme.Overlay`
border), and [safe-presentation.md](safe-presentation.md) (the escaped
single-line path).

## Lifecycle (`internal/app/popup.go`)

- **Starts at selection, not load completion.** `navigate` returns
  `tea.Batch(m.startLoad(), m.startPopup())` on a `FileChanged` step,
  so the pop-up is live while the destination may still be loading —
  and is the only command issued when the destination is already
  cached, in flight, or failed (`tea.Batch` compacts nil leaves).
  `fileLoadedMsg` never touches the pop-up: load completion neither
  dismisses it nor restarts its timer. Strict no-op steps — an empty
  or single-stop index — open no pop-up.
- **Instance-keyed one-second lifetime.** `startPopup` mints a fresh
  ID from `popupSeq` into `popupID` (0 means none up) and returns a
  `tea.Tick(popupLifetime)` — one second — command producing
  `popupExpireMsg{id}`. `Update` dismisses the live pop-up only when
  the message's `id` equals `popupID`, so an expiry minted for an
  older instance is stale and discarded — a late timer can never
  dismiss a newer pop-up.
- **Any key dismisses and still acts.** `Update` clears `popupID`
  before the ordinary key switch, so the same press that dismisses the
  pop-up also navigates, scrolls, toggles the theme, or quits — the
  pop-up never delays input. `Esc` in ordinary browsing is a no-op
  beyond the dismissal.
- **Error-overlay cancellation.** `openOverlay` clears `popupID`, so
  an overlay arriving while a pop-up is up cancels it permanently — no
  pop-up returns after the overlay is dismissed. Help-overlay
  cancellation and combined-precedence testing are owned by Issues #31
  and #32.
- **Test seam.** `options.popupTimer`, when set, builds the expiry
  command in place of the real tick — model tests substitute an
  instantly resolving command and drive expiry by injecting
  `popupExpireMsg` with an explicit ID; no sleeps.

## Rendering (`compositePopup`)

- Content is the current file's path through
  `safepresentation.EscapePath` — a single sanitized line hostile
  bytes cannot break — shown as the sole interior row of a
  `theme.Overlay` bordered box, the same border the error overlay
  uses.
- Centring and truncation are computed from the current terminal size
  at **every render**, with no state mutation in `View`: the path is
  left-truncated with a leading `…` (`leftTruncate`/`tailCells`, whole
  grapheme clusters only) so the basename end stays visible, and the
  box centres at `((h − boxRows) / 2, (w − boxW) / 2)`, clamped at the
  top-left when it exceeds the frame. A resize therefore recentres and
  re-truncates the same live instance without restarting its timer.
- **Precedence**: `View` composites the pop-up over the base frame and
  the error overlay over both — the modal overlay always wins while
  open.

## Tests

`internal/app/popup_test.go` (same package) drives the whole contract
through `Update` with the `popupStubTicks` options seam and
batch-unwrapping `leafMsgs`/`navLeafMsgs`/`deliverLoad` helpers:
selection-time start over the loading panel with load completion
leaving the instance untouched, centring on the frame, stale-instance
rejection and own-expiry dismissal, key-press dismissal plus normal
action (scroll key scrolls, `q` quits, `Esc` only dismisses), resize
recentring and re-truncation with the same instance answering its
expiry, error-overlay cancellation with no return, left-truncated long
paths, and the escaped hostile path never adding rows.
`sinksafety_test.go` gains the file-change pop-up row driven through
`popupFixtureView`. `browse_test.go`'s `finishLoad` now unwraps
`tea.BatchMsg` (`fileLoadOf`) to deliver the load leaf, and the
pre-#15 "cached file issues no command" assertions in `nav_test.go`
and `reveal_test.go` became "no load leaf" checks under the stub timer
(`navSendsNoLoad`). See [unit-tests.md](unit-tests.md).
