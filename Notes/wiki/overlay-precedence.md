# Overlay precedence and dismissal semantics (Issue #32)

Issue #32 established the full precedence stack, error-suspends-help
with scroll restoration, the generalized append-preserving-scroll
primitive, pop-up cancellation by help and error, the `Esc` no-op
rule, and the dismissal-outcome table for both `q` and `Esc`.
Relevant PRD sections: *Colours, overlays, and key precedence*,
*Outcome and exit-status contract*. See also
[help-overlay](help-overlay.md), [outcome-contract](outcome-contract.md),
[read-failures-and-retry](read-failures-and-retry.md), and
[browse-tracer](browse-tracer.md).

## Precedence stack

In ordinary browse and no-results states, the key-precedence stack is:

1. `ctrl+c` — global precedence, exits 130 from any state, overriding
   the fixed search-derived exit status.
2. Modal error overlay — captures all keys when an error or warning
   overlay is open.
3. Help overlay — captures all keys when help is open (unless suspended
   by an error).
4. File-change pop-up — any key dismisses the pop-up and performs its
   normal action in the same update.
5. Base-state keys — `n`/`p`/`r`/`w`/`c`/scroll/pan/list-toggle/`q`.

The too-small screen's dedicated rule is owned by Issue #33.

## Error suspends help

A new error while help is open at scroll position S suspends help
rather than closing it. The error overlay replaces help on screen;
help's scroll position is saved in `suspendedHelpScroll`. Either
dismissal key (`q` or `Esc`) dismisses the error and restores help at
S — the reader returns to the exact scroll position they were at when
the error interrupted.

While the error is suspended over help, keys route to the error
(error-first key routing): scrolling affects the error overlay's
scroll, not the suspended help's saved position. Dismissing the error
restores help at its original scroll position, not the error's.

Opening help clears any suspended-help state from a previously
dismissed error-over-help sequence, so a fresh help open always starts
at scroll 0.

## Append preserves scroll

Issue #26 introduced the append-preserving-scroll primitive for the
re-entry retry failure case: when a read-failure overlay is open and a
re-entry retry fails again, the new diagnostic is appended to the
overlay text without resetting the scroll position. Issue #32
generalized this to all appended errors: a read failure while any
error or warning overlay is open appends the new diagnostic without
resetting the scroll position.

The reader who has scrolled to position P stays at P after the
append. The new text is reachable by scrolling down. The overlay is
marked as a read-failure overlay (`overlayReadFailure = true`) so `r`
(Issue #27) can retry through it.

This replaces the Issue #26 rule that a search-complete overlay takes
precedence over read failures. A read failure while a search-complete
error or warning overlay is open now appends to it rather than being
suppressed.

## Pop-up cancellation

Opening help or an error overlay cancels any active Issue #15
file-change pop-up. After cancellation the pop-up does not return when
the overlay is dismissed. This applies to:

- Help overlay opened with `h`/`?` (Issue #31).
- Error overlay opened by a read failure (Issue #26).
- Error overlay opened by a search-complete fatal/warning outcome
  (Issue #9).

The pop-up is cancelled (not merely dismissed): the instance is
cleared so a stale timer cannot revive it.

## `Esc` no-op

`Esc` is an overlay-dismissal key only. With no help or error overlay
open — during searching, on the no-results screen, and in normal
browsing – `Esc` is a no-op. Its only effect is dismissing a pop-up
(like any other key); beyond that it does nothing.

`Esc` never exits from a base state. The one exception is dismissing a
fatal no-results overlay with `Esc`: because there is no underlying
state, `Esc` terminates with status 2.

## Dismissal-outcome table

The dismissal-outcome table for both `q` and `Esc` as the dismissal
key:

| Base state | Overlay | After dismissal | Second `q` | Second `Esc` |
|---|---|---|---|---|
| browse | error | browse (still running) | exits fixed status | no-op |
| browse | help | browse (still running) | exits fixed status | no-op |
| browse | error over help | help restored (still running) | closes help → browse; further `q` exits | closes help → browse; `Esc` no-op |
| no-results | warning | no-results (still running) | exits 1 | no-op |
| no-results | help | no-results (still running) | exits 1 | no-op |
| no-results | fatal error | exit 2 | — | — |
| no-results | record-loss error | exit 2 | — | — |

For the error-over-help row, the full three-key sequence is:

1. The dismissal (`q` or `Esc`) closes the error and restores help at
   its saved scroll position.
2. The next `q` or `Esc` closes the restored help to browsing.
3. Only a further base-state `q` exits with the fixed status.

`Esc` never exits from a base state. Dismissing a fatal no-results
overlay with `Esc` terminates with status 2 because there is no
underlying state.

## Model fields

- `suspendedHelp bool` — help has been suspended by a modal error
  overlay. Dismissing the error restores help at `suspendedHelpScroll`.
- `suspendedHelpScroll int` — the saved scroll position of the
  suspended help overlay.

## Public accessors

- `HelpSuspended() bool` — reports whether help has been suspended by
  a modal error overlay (Issue #32).

## Out of scope

- The too-small screen's dedicated rule is owned by Issue #33.
- The help overlay's binding table, footer slot, and tiny-size
  clipping are owned by Issue #31.
- The error overlay's outcome matrix and exit-status contract are
  owned by Issue #9.

## Cross-references

- [Issue #32](../issues/032-overlay-precedence-esc-semantics.md)
- [PRD: Colours, overlays, and key precedence](../PRD-vrg.md)
- [PRD: Outcome and exit-status contract](../PRD-vrg.md)
- [Help overlay (Issue #31)](help-overlay.md)
- [Outcome contract (Issue #9)](outcome-contract.md)
- [Read failures and retry (Issue #26)](read-failures-and-retry.md)
- [Browse tracer (Issue #15 pop-up)](browse-tracer.md)
- [Too-small screen (Issue #33)](too-small-screen.md) — the too-small
  `q` rule takes precedence over the Issue #32 dismissal semantics
