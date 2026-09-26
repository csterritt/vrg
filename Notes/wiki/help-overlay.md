# Help overlay — `h`/`?` modal key bindings (Issue #31)

The modal help contract delivered by
[Issue #31](../issues/031-help-overlay.md), implemented in
`internal/app` (`help.go`, with the shared overlay component in
`overlay.go` and wiring in `app.go`). Relevant PRD section in
`Notes/PRD-vrg.md`: *Colours, overlays, and key precedence* (the help
bullet, the clipping bullet, and the precedence line — in ordinary
browse/no-results states: `ctrl+c`, then modal error, then help, then
pop-up, then base-state keys).

## Opening and closing

`h` and `?` open the modal help overlay over ordinary browsing and over
the no-results screen — the two states the PRD's modal-precedence model
covers. During searching they are inert like every other ordinary key
(only `q` and `ctrl+c` act there). `openHelp` builds an `overlay` value
holding `helpLines()`' composed text and clears `popupID`: opening help
cancels an active Issue #15 file-change pop-up, and it does not return
when help closes — the same no-return rule `openOverlay` applies for
errors.

Closing returns to the underlying base state unchanged: over browse,
browsing resumes; over no-results, the centred screen returns and a
subsequent `q` still exits 1. `q`, `Esc`, `h`, and `?` all close help;
help never quits outright — unlike the fatal-only diagnostics overlay
it always has an underlying state.

## Key contract

`helpKey` runs in `Update` after the diagnostics-overlay check and
before the base-state switch, matching the error > help > pop-up >
base precedence:

- `up`/`down` scroll the rendered rows, clamped at both ends through
  the shared `scrollOverlay` helper.
- `q`/`Esc`/`h`/`?` close.
- `ctrl+c` takes the cancellation path to exit 130.
- Every other key is ignored — `n`/`p`/`w`/`c`/`r` and the rest never
  reach the cursor, the wrap or colour toggles, the load state, or the
  screen behind the modal.

An error overlay opening while help is up suspends help rather than
destroying it: `m.help` keeps its lines and scroll offset, `m.overlay`
takes the keyboard and the topmost render, and dismissing the error
reveals help at its retained position. See
[overlay-precedence.md](overlay-precedence.md) for the combined
precedence and `Esc`-semantics matrix over this stack (Issue #32).

## The binding table and footer slot

`helpBindings` in `help.go` is the single binding table — a `[]helpBinding`
of key spelling → description covering navigation (`n`/`p`), scrolling
(`up`/`down`, `u`/`d`, `pgup`/`pgdown`), panning (`,`/`.`, `<`/`>`,
`[`/`]`), wrap (`w`), colour (`c`), the list toggle (`left`/`tab`,
`right`/`shift+tab`), reload (`r`), help (`h`/`?`), and quit/cancel
(`q`, `Esc`, `ctrl+c`). `helpLines` renders the title `Key bindings`,
one row per entry (keys padded to the widest spelling), then the
**footer**: `helpFooter`, filled since Issue #34 with `limitNotes` —
the three scale/record-limit/memory statements that also appear
verbatim in `README.md` (see
[documentation.md](documentation.md)). The footer is the substitution
point — `helpLines` routes each entry through `present.Diagnostic`,
the Issue #6 utility, so runtime text can never emit control bytes
into the overlay; the binding table itself is static text. Issue #34's
documentation tests iterate `helpBindings` against the README and
assert every `helpFooter` entry appears verbatim in both sinks, so
neither rendered key list nor note can drift from the implementation.

## Shared component and rendering

The Issue #9 `overlay` struct — sanitized lines plus a wrapped-row
scroll offset — is now the shared component both modals instantiate.
Its geometry helpers became value methods, `layout(w, h)` and
`maxScroll(w, h)`, taking the frame dimensions; `scrollOverlay` is the
shared clamped up/down handler; `renderOverlay(base, o)` composites
whichever instance is passed. `View` paints in precedence order — pop-up,
then help, then the diagnostics overlay — so a suspended help stays
visible beneath the error that interrupted it.

Wrapping and clipping are inherited from the shared component: lines
hard-wrap with `ansi.Wrap` to the interior width (frame minus the
border, capped at the widest line), so long unbroken strings occupy
several interior rows rather than overflowing; vertical scrolling
reaches every wrapped row at usable sizes. At tiny sizes above the
20×3 minimum there is no borderless mode — `composite` clips the box to
the frame and growth restores the normal layout; below the minimum the
Issue #33 too-small gate replaces the whole frame and the overlay's
scroll position survives the round trip untouched (see
[terminal-too-small.md](terminal-too-small.md)). The overlay uses
`theme.Overlay`'s single-line border painted in the base colours (see
[theme-and-colour-toggle.md](theme-and-colour-toggle.md)).

## Sink safety

The help overlay is a new output sink and carries its
`sinkSafetySinks` row (`help overlay`): `renderHelpSink` drives the
hostile fixture bytes through the footer substitution slot — the only
runtime-text route help has — opens the overlay over browse with `?`,
and asserts the `Diagnostic`-escaped `wantDiag` forms inside the
border under the shared no-style and styled passes. Issue #34 added a
second row, `help footer note` (`renderHelpFooterSink`): the fixture
is substituted at every runtime-substitution point of the rendered
footer — appended to each installed `limitNotes` entry plus a
dedicated injection line — with the view scrolled to the footer's
rows, and the check asserts the escaped forms alongside the real
note's `64 MiB` text. See
[safe-presentation.md](safe-presentation.md).

## Tests

`internal/app/help_test.go` drives `Update` with injected messages:
`TestHelpOpensFromBrowse` / `TestHelpOpensFromNoResults` (`h` and `?`
from each openable state, close returning to the base state, `q` on
no-results still exiting 1), `TestHelpCloseKeys` (all four close keys),
`TestHelpIgnoresOtherKeys` (including `n`/`p`/`w`/`c`/`r` — cursor,
wrap, theme, and load state unchanged), `TestHelpCtrlCExits130`,
`TestHelpKeysInertWhileSearching`, `TestHelpScrollsWithUpDown` (clamped
scrolling reaching head and tail at a reduced height),
`TestHelpWrapsUnbrokenSubstitution` (a 300-cell unbroken footer string
wraps inside the border), `TestHelpClippedAtTinySize` (no panic and a
clipped-but-present box at 25×8, normal layout restored on growth),
`TestHelpRendersBindingTable` and `TestHelpRendersFooterSlot` (every
`helpBindings` row and the footer text appear in the render), and
`TestHelpCancelsPopup` (pop-up cancelled on open, no return on close,
stale expiry inert). `sinksafety_test.go` gained the `help overlay`
row and, since Issue #34, the `help footer note` row. Issue #34's
`readme_test.go` adds the binding-table and footer-note README
synchronization tests — see
[documentation.md](documentation.md) and [unit-tests.md](unit-tests.md).

## Files

- `internal/app/help.go` — `helpBinding`, `helpBindings`, `limitNotes`,
  `helpFooter`, `helpLines`, `openHelp`, `helpKey`.
- `internal/app/overlay.go` — the `overlay` component generalized:
  `layout`, `maxScroll`, `scrollOverlay`, `renderOverlay(base, o)`;
  `overlayKey` keeps the diagnostics-specific dismissal (the fatal
  no-results overlay quits outright).
- `internal/app/app.go` — the `help` field, the `h`/`?` browse and
  no-results case, the help tier in `KeyPressMsg` routing, and `View`'s
  pop-up → help → diagnostics compositing order.

See also:
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
(the other instance of the shared component),
[file-change-popup.md](file-change-popup.md) (the pop-up help cancels),
[no-results-and-binary-exclusion.md](no-results-and-binary-exclusion.md)
(the other state help opens from),
[safe-presentation.md](safe-presentation.md) (the substitution utility
and the sink row).
