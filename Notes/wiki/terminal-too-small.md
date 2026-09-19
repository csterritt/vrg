# "Terminal too small" gate with full state recovery (Issue #33)

Delivered by
[Issue #33](../issues/033-terminal-too-small-with-state-recovery.md)
([task](../tasks/033-terminal-too-small-with-state-recovery.md)): below
the fixed terminal minimum of **20 columns or 3 rows** the whole
ordinary presentation and key map are replaced by a centred "Terminal
too small" note, and growing back restores the session exactly — a
degradation boundary, not a state transformation. Relevant PRD section:
*Layout and indicators* (the minimum-size bullet) in
[`Notes/PRD-vrg.md`](../PRD-vrg.md); user story 32. Builds on
[overlay-precedence.md](overlay-precedence.md) (the modal stack the
gate suspends rather than disturbs),
[file-change-popup.md](file-change-popup.md) (the pop-up whose timer
runs on behind the gate),
[logical-anchor.md](logical-anchor.md) (the anchors and keyed layout
pipeline the gate must not touch), and
[file-list-layout.md](file-list-layout.md) (the widths the gate
replaces).

## The gate

`tooSmall()` reports `m.sized && (m.width < 20 || m.height < 3)`. The
`sized` flag matters: before any `WindowSizeMsg` arrives the model is
unsized — the gate stays off so the app still opens on "Searching…"
— while a *reported* sub-minimum size engages it. The gate installs at
three points:

- **`WindowSizeMsg`** records the dimensions and `sized`, then returns
  early: no modal scroll clamping, no layout re-request, no anchor or
  viewport work at pathological dimensions. A resize *wholly within*
  the gate — 19×2 → 10×1 — therefore mutates nothing; the first size
  back above both minimums runs the ordinary resize path (clamp
  overlays, re-request the current file's layout, restore viewports) at
  the **final** dimensions.
- **`layoutReadyMsg`** discards completions arriving while the gate is
  up — even one minted before it engaged — so no ordinary layout is
  ever installed sub-minimum; recovery re-requests at the size that
  lifts it.
- **`requestLayout`** returns nil while gated, so no new ordinary
  layout is prepared either.

The gate is strictly boundary logic: it never closes an overlay, never
mutates an anchor, cursor, scroll position, or preference, and never
discards pop-up identity. Preserved state is simply never touched, so
recovery needs no restoration logic beyond the ordinary resize path.

## Presentation

`View` short-circuits before composing anything else:
`s = center(clipCells("Terminal too small", m.width), m.width,
m.height)` — the note clipped to a narrower frame (at 10 columns the
row is `Terminal t`) and vertically centred in whatever height exists.
The state screen, modal stack, and pop-up are **not composited** — the
too-small screen shows the note only, which is also why a logically
open overlay is invisible behind it.

## Key map

`KeyPressMsg` routes to `tooSmallKey` before the pop-up dismissal and
the ordinary precedence stack. Only two keys act:

- **`ctrl+c`** — the global override holds: exit 130 from every state.
- **`q`** — exits with the state-applicable outcome: 130 while
  searching, otherwise the status the completed search fixed — the
  fixed browse status (0 or 2), 1 on the no-results screen, 2 with the
  fatal overlay logically open. With a browse error overlay logically
  open `q` still **exits** with the underlying fixed status rather than
  merely dismissing — this precedence deliberately beats
  [Issue #32's dismissal semantics](overlay-precedence.md) so the
  screen can never trap the user behind an invisible modal.
- **`Esc` and every other key are strict no-ops** — no pop-up
  dismissal, no navigation, no toggles, no modal change. `Esc` in
  particular never dismisses the hidden overlay, so it is still
  logically open (at its retained scroll) when the terminal grows.

## Preserved state

Everything behind the gate survives a round trip unchanged, all proven
by the model tests:

- cursor selection and the current file;
- per-file viewport state and the logical `(line, cell)` anchors —
  including mid-wrapped-line anchors across a relayout at the new size;
- the file-list visibility preference, wrap mode, colour scheme, and
  horizontal pan offset;
- the exact modal stack: which overlay is open, the help scroll
  position, the error-overlay scroll position, and the
  help-suspended-by-error relationship (a scrolled error over scrolled
  help returns both at their positions);
- the pending pop-up's identity — see below.

## Pop-up behaviour

`popupExpireMsg` handling is **not** gated: a live pop-up's timer keeps
running behind the gate and an expiry that lands during too-small
dismisses its instance permanently — it does not reappear after
recovery. A still-live pop-up at lift-off reappears in the recovered
frame (recomputed at the new size, as always). The pop-up is simply
never composited while gated — consistent with
[file-change-popup.md](file-change-popup.md)'s render-at-View model.

## Fixture notes

The gate changes the meaning of sub-minimum frames in tests:
`popup_test.go`'s resize-recentring case moved from 12×5 to 20×5 (the
smallest ordinary frame), and `hreveal_test.go`'s unpaintable-cluster
fixture — which needs a 9-column frame for a text area narrower than a
tab expansion, impossible at the 20-column minimum — now sets the
model's dimensions directly and returns it to unsized, exercising the
reveal arithmetic at the only model level where a sub-minimum frame
can still exist.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/app/toosmall_test.go` drives the whole contract through
`Update` — the 20×3 threshold and centred/clipped note, the per-state
`q` exit-status table including the overlay-open rows, `ctrl+c` → 130,
`Esc` and the ignored-key sweep, full modal-stack and browse-state
round trips, the 19×2 → 10×1 → 25×8 in-gate resize deferring all
layout and modal work to the final dimensions, and pop-up expiry and
survival across the gate.
