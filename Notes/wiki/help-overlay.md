# Help overlay — the `h`/`?` modal binding table (Issue #31)

Delivered by
[Issue #31](../issues/031-help-overlay.md)
([task](../tasks/031-help-overlay.md)): `h` and `?` open a modal help
dialog listing every key binding — a bordered, wrapped, scrollable box
over the browse view or the no-results screen — and closing returns to
the underlying base state. Relevant PRD section: *Colours, overlays,
and key precedence* (the help bullets and the clipped-overlays bullet)
in [`Notes/PRD-vrg.md`](../PRD-vrg.md); user stories 78, 79, 82, 83.
Builds on
[error-overlay-and-outcomes.md](error-overlay-and-outcomes.md) (the
shared scrollable overlay component),
[file-change-popup.md](file-change-popup.md) (pop-up cancellation),
[theme.md](theme.md) (the `theme.Overlay` border), and
[safe-presentation.md](safe-presentation.md) (the escaped footer
substitution).

## The shared wrapped, scrollable component

`internal/app/overlay.go` now factors the Issue #9 overlay body into
`scrollBox` — `text` re-wrapped into interior-width rows on every
render (`scrollBox.rows` over `wrapCells`: grapheme-cluster boundaries,
unbroken strings split mid-run) and `scroll`, the first visible wrapped
row clamped to the complete row set (`scrollBox.scrollBy`/`clamp`). The
error overlay and the help dialog each own one (`m.overlay`,
`m.help`), and `compositeBox` draws either: the `theme.Overlay`
single-line bordered box centred on the frame carrying the visible
window of wrapped rows in the base colours. At tiny sizes above the
20×3 minimum the box clips to the terminal — there is **no borderless
mode** — and growth restores the normal layout.

## Key routing (`internal/app/app.go`)

- **Open.** `h`/`?` open help from `stateBrowse` and `stateNoResults`
  alike; `openHelp` composes the body and clears `popupID`, so a live
  file-change pop-up is cancelled and never returns after help closes.
- **Modal while open.** `up`/`down` scroll one rendered row each,
  clamped to `[0, rows − visible]`; `q`, `Esc`, `h`, and `?` all close
  back to the base state — closing over no-results still leaves `q`
  exiting 1; `ctrl+c` keeps the global 130 override; every other key —
  including `n`, `p`, `w`, `c`, and `r` — is ignored with the state
  behind untouched.
- **Precedence.** The `helpOpen` case sits between the `overlayOpen`
  case and the base-state keys: `ctrl+c` over modal error over help
  over pop-up over base keys. An error opening while help is up
  suspends it — `helpOpen` and `help.scroll` are retained, and
  dismissing the error restores help at its position; Issue #32
  verifies the combined stack in
  [overlay-precedence.md](overlay-precedence.md). Issue #27's `r` route is
  guarded by `!m.helpOpen`: the explicit reload still fires under an
  error overlay but never under help.
- **View order.** `View` composites pop-up, then help, then the error
  overlay — an open error always draws on top.

## The binding table (`internal/app/help.go`)

`helpBindings` is the **single binding-table data source**: one
`{keys, desc}` row per binding — navigation (`n`/`p`), scrolling
(`up`/`down`, `u`/`d`, `pgup`/`pgdn`), panning (`,`/`.`, `<`/`>`,
`[`/`]`), the file-list toggles (`left`/`tab`, `right`/`shift+tab`),
wrap (`w`), colour (`c`), reload (`r`), help (`h`/`?`), and
quit/cancel (`q`/`esc`, `ctrl+c`). `helpText` renders it as a "Key
bindings" title row plus one `keys  desc` row per binding, the key
sets padded to a column. Issue #34's documentation test iterates the
same table so the README cannot drift from the routed keys.

`helpFooter` is the reserved **footer slot** under the table — empty
until Issue #34 fills it with the scale-and-limits note. It is
substituted text from the renderer's perspective: `helpText` routes it
through `safepresentation.EscapeDiagnostic`, so whatever fills the slot
can never emit control bytes — the sink-safety table's "TUI help
dialog" row drives the hostile fixture set through that substitution.

## Tests

`internal/app/help_test.go` (same package; Issue #31) covers the whole
contract: `h`/`?` opening from browse and from no-results, each of the
four close keys returning to the underlying base state with a
subsequent `q` on no-results exiting 1, the ignored-keys sweep leaving
cursor/viewport/wrap/theme/list/loading untouched, `up`/`down` scroll
bounds over the complete wrapped table, `ctrl+c` exiting 130, a
200-cell unbroken footer substitution wrapping within the border, the
25×8 clipped-but-bordered frame and its restoration on growth, the
binding-table rendering iterating `helpBindings`, and pop-up
cancellation with no return. `sinksafety_test.go` gains the "TUI help
dialog" row driven through `helpFixtureView`. See
[unit-tests.md](unit-tests.md).
