# Destination reveal — vertical placement of the navigation target

Issue #14
(`Notes/issues/014-vertical-destination-reveal.md`, tasks
`Notes/tasks/014-vertical-destination-reveal.md`) added the vertical
destination reveal: on startup once content is loaded and on every
actual `n`/`p` transition, the rendered row containing the destination
match is made visible under a fixed placement rule.

PRD cross-references: "Navigation, viewport, and logical anchors" (the
target-row and placement bullets — the display target, the
visible-target no-scroll, the one-third placement with BOF/EOF
precedence, and the saved-viewport versus top-of-file starting points)
and "Testing Decisions → Viewport" in `Notes/PRD-vrg.md`.

## The display target

The reveal target is a *display location*, not a source-line ordinal:
`viewport.Target{Line, Cell}` carries the zero-based source line and the
display cell of the destination line's **first submatch's start** — the
marker cell when the submatch is zero-width. `Model.reveal`
(`internal/app/browse.go`) computes it from the buffer's validated
spans: the smallest `Start` among the destination line's spans, so a
stale-content drop automatically falls through to the first surviving
submatch (the remaining stale-entry fallbacks — clamped recorded starts
and gone-line landing — are Issue #29's).

The viewport never assumes a target's rendered row equals its line:
`viewport.Rows` gained `RowOf(Target) int`, answered by the prepared
row model, so wrap mode maps the target cell into the correct one of a
source line's many rendered rows. Issue #16's `viewport.Model` answers
with the line itself in run-off-edge mode and with the row covering
the target cell in wrap mode — boundary positions belong to the next
row, an end-of-line marker lands on its own trailing row (see
[wrap-mode.md](wrap-mode.md)).

## The placement rule

`Viewport.Reveal(Target) bool` (`internal/viewport/viewport.go`)
implements the contract:

- **Visible target → no scroll.** A target row inside
  `[top, top+height)` leaves the top untouched and reports no movement.
- **Hidden target → one-third placement.** The top moves to
  `target row − floor(content height / 3)`, so the destination lands
  about a third down the content area, in either direction.
- **BOF/EOF precedence.** The resulting top still clamps to
  `[0, max(0, count − height)]`: near the head of the file the target
  sits higher than the one-third row; near the tail it lands lower —
  available content wins over exact placement.
- **The move report drives saved state.** `Reveal` returns whether the
  viewport actually moved; the app replaces the file's `saved` top only
  on a move, so a no-scroll reveal leaves saved state untouched and a
  moving reveal overwrites it.
- **No content → no-op.** With no prepared rows (the `Loading…` and
  `(unreadable)` placeholders) the reveal is inert — matching the
  placeholder scroll no-op.

## The starting-viewport sequence on entry

Entering a file — including the startup file — always runs the same
sequence, per the PRD's file-change rule:

1. **Start from the saved per-file viewport** on a revisit
   (`SetTop(m.saved[path])`), or from the **top of the file** on a
   first visit — the absent map entry reads as 0.
2. **Apply the destination reveal** over that starting point: a saved
   position that already shows the target survives untouched; one that
   hides it is overridden by the one-third placement.

`navigate` runs this inline for a file crossing whose destination is
already cached — saved top installed, then `reveal()`. When the
destination is uncached or in flight, its rows are absent and the
reveal no-ops; the `loadDoneMsg` path runs the identical sequence when
the buffer arrives (`SetTop` of the saved state, then `reveal()`),
which is also the startup-after-load trigger — and because `reveal`
reads the live cursor, a load completing after further navigation
reveals the *latest* selected stop, the behavior Issue #28 formalizes.
A same-file step keeps the current viewport and simply reveals the new
stop against it.

Horizontal reveal of the target cell is Issue #19's; nothing here
pans left or right.

## Tests

`internal/viewport/reveal_test.go` pins the placement contract against
the counting fake plus `mappingRows`, a programmable `RowOf` fake that
proves the reveal consumes the provider's rendered row and sees the
exact `(line, cell)` target: visible-target no-scroll at window edges,
one-third placement above and below, BOF/EOF clamp precedence,
out-of-range target clamping, inert empty content, and the
saved-versus-first-visit starting sequence. `internal/app/reveal_test.go`
drives the triggers through `Update`: startup reveal after load
(including gate-held load completion), `n`/`p` placement and the
saved-state replacement, the on-screen `n` no-scroll, revisit starting
from the saved viewport, and the first-visit top-then-reveal order.
See [unit-tests.md](unit-tests.md) § `internal/viewport` and
`internal/app`.

## Files

- `internal/viewport/viewport.go` — `Target`, `Rows.RowOf`, and
  `Viewport.Reveal`.
- `internal/app/browse.go` — `reveal` and the `navigate` reveal
  trigger.
- `internal/viewport/rows.go` — `Model.RowOf`, the wrap-aware answer
  (Issue #16).
- `internal/app/app.go` — the `loadDoneMsg` saved-state-plus-reveal
  sequence for the current path.

See also: [match-navigation.md](match-navigation.md) (the cursor steps
that trigger the reveal),
[viewport-scrolling.md](viewport-scrolling.md) (the clamped top and
per-file saved state the reveal consumes and replaces),
[browse-tracer.md](browse-tracer.md) (the file panel it scrolls), and
[searchindex-records-and-stops.md](searchindex-records-and-stops.md)
(the stop/submatch data the target derives from).
