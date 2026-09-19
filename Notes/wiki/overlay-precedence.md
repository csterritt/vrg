# Overlay precedence and `q`/`Esc` dismissal semantics (Issue #32)

[Issue #32](../issues/032-overlay-precedence-esc-semantics.md)
([task](../tasks/032-overlay-precedence-esc-semantics.md)) pins the
composed precedence stack the earlier overlay issues built: it is a
verification issue whose model tests prove the interactions of
[Issue #15's pop-up](file-change-popup.md),
[Issue #26's read-failure appends](read-failures.md), and
[Issue #31's help dialog](help-overlay.md) against the modal
[error overlay](error-overlay-and-outcomes.md) — no production code
was needed, since the predecessors' implementation already satisfies
the contracts. Relevant PRD sections: *Colours, overlays, and key
precedence* and *Outcome and exit-status contract* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 80, 81, 84.

## The precedence stack

`Update`'s key routing descends a strict order — `ctrl+c` over the
modal error overlay over modal help over the pop-up over base keys:

1. `ctrl+c` — the global override: begins controlled exit and exits
   130 no matter what is open, including over an error-over-help
   stack.
2. `overlayOpen` — the modal error overlay: `up`/`down` scroll,
   `q`/`Esc` dismiss, everything else ignored (except Issue #27's
   `r` route, placed before this case so an explicit reload still
   fires under a failure overlay).
3. `helpOpen` — the modal help dialog: `up`/`down` scroll,
   `q`/`Esc`/`h`/`?` close, everything else ignored.
4. `popupID ≠ 0` — the file-change pop-up is cleared before the key
   switch on every press, so the same key both dismisses it and
   performs its ordinary action.
5. Base-state keys — searching cancellation, then the ordinary
   browse/no-results bindings.

Because the error case precedes the help case, an overlay open over
help receives every key first: an `up`/`down` while the stack is up
scrolls the **error**, not the suspended help.

## Error suspends help

`helpOpen` and `help.scroll` are never cleared by an error's arrival:
`openOverlay` leaves them alone, `View` composites pop-up then help
then the error overlay, and the error's earlier routing case keeps
keys off help while it is up. Dismissing the error — with either `q`
or `Esc` — restores help at its retained scroll position. A second
error arriving while help is suspended appends to the open error
overlay rather than re-opening it, so the suspended help state is
still untouched.

## Append-preserving scroll

`openOverlay` is one open-or-append entry point: on an open overlay it
joins the new diagnostic under the old (`overlay.text += "\n" + text`)
without touching `overlay.scroll`, so the reader's position holds and
the appended text is reachable by scrolling; on a closed overlay it
starts fresh at scroll 0. Issue #26 owns the primitive for the
read-failure retry case; Issue #32 generalizes the contract to all
appended errors.

## Pop-up cancellation

Both `openOverlay` and `openHelp` clear `popupID`. A file-change
pop-up up when an error or help opens is cancelled permanently — the
stale `popupExpireMsg` its minted timer still produces is rejected by
the instance-ID check, so no suspended pop-up returns after the
overlay or help is dismissed.

## The dismissal-outcome table

`q` and `Esc` are interchangeable dismissal keys in every modal. The
composed table — each row proven for both keys:

| State at key press | First `q`/`Esc` | State after | Then `q` exits | Then `Esc` |
| --- | --- | --- | --- | --- |
| browse + error overlay | dismisses overlay | browse, running | fixed status | no-op |
| browse + help | closes help | browse, running | fixed status | no-op |
| browse + error over help | dismisses error | help restored at saved scroll | — | — |
| no-results + warning overlay | dismisses overlay | no-results, running | 1 | no-op |
| no-results + help | closes help | no-results, running | 1 | no-op |
| fatal overlay, no usable results | exits | — | — | — |
| record-loss overlay, no results | exits | — | — | — |

The fatal rows have `overlayExit` set (the Issue #9 `dismiss-exits`
flag): with no underlying state to return to, dismissal is the exit —
status 2 for both keys, making `Esc`'s exit the single exception to
its never-exits rule. For the error-over-help row the sequence
continues: a second `q`/`Esc` closes help to browse, and only a
further base-state `q` exits.

## The too-small exception

Issue #33's gate adds one deliberate exception to this table: below
the 20×3 minimum the modal stack is invisible but logically intact,
and `q` **exits** with the state-applicable status instead of
demoting to a dismissal — the screen can never trap the user behind a
hidden modal. `Esc` stays a strict no-op there (it does not dismiss
the hidden overlay either). See
[terminal-too-small.md](terminal-too-small.md).

## `Esc` never exits a base state

`Esc` is an overlay-dismissal key only: no base-state routing case
matches it, so in ordinary browsing and on the no-results screen it
is a strict no-op — state, status, and commands all unchanged. The
one place `Esc` terminates is the fatal-overlay dismissal above,
where it exits 2 precisely because there is no underlying state.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/app/precedence_test.go` drives the whole table through
`Update` — error-suspends-help with scroll restoration for both
dismissal keys, appended-error scroll preservation and reachability,
pop-up cancellation by help and by error, `Esc` no-ops in browse and
no-results, the full dismissal-outcome table run for `q` and `Esc`,
error-first key routing over the stack, and `ctrl+c`'s 130 override
above it.
