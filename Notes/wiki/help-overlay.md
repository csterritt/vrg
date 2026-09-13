# Help overlay (Issue #31)

The modal help overlay opened with `h`/`?` from ordinary browsing and
the no-results screen. It uses the same wrapped, scrollable overlay
component as the [Issue #9 error overlay](outcome-contract.md) with base
colours and a plain single-line border. Closing returns to the
underlying base state.

## Opening

`h` or `?` opens the help overlay from the browse state and the
no-results state. Opening help cancels any active [Issue #15
file-change pop-up](browse-tracer.md) with no return on close — the
pop-up is cancelled (not merely dismissed), so a stale timer cannot
revive it and it does not reappear when help is closed.

## Key routing while open

While the help overlay is open:

| Key | Action |
|---|---|
| `up`/`down` | Scroll rendered rows |
| `q` | Close help, return to base state |
| `Esc` | Close help, return to base state |
| `h`/`?` | Close help, return to base state |
| `ctrl+c` | Exit 130 (global precedence) |
| any other key | Ignored — underlying state unchanged |

Every key other than `up`/`down`/`q`/`Esc`/`h`/`?`/`ctrl+c` — including
`n`/`p`/`w`/`c`/`r` — is ignored while help is open. The state behind the
overlay (browse or no-results) is unchanged. This differs from the
error overlay, where `h`/`?` are ignored (they do not close the error
overlay) and `r` is allowed through a read-failure overlay (Issue #27).

`ctrl+c` has global precedence and exits 130 from any state,
overriding the fixed search-derived exit status.

## Closing

Closing the help overlay (via `q`/`Esc`/`h`/`?`) returns to the
underlying base state:

- From browse: returns to browse.
- From no-results: returns to no-results; a subsequent `q` still exits 1.

The help overlay is never fatal: closing it never exits.

## Shared overlay component

The help overlay shares the `renderOverlay` component with the [Issue
#9 error overlay](outcome-contract.md):

- Base colours (dark: white on black; light: black on white) from the
  [theme](theme-module.md).
- Plain single-line border (box-drawing chars).
- Text wraps to the interior width (accounting for border sides and
  margins), including long unbroken strings.
- Vertical scrolling reaches every row at usable sizes.

The help overlay skips the head/tail compression that the error
overlay applies to very large diagnostics. This ensures vertical
scrolling reaches every row of the binding table rather than
compressing the middle.

## Tiny-size clipping

At tiny terminal sizes the overlay is clipped to the terminal without a
special borderless mode:

- The overlay width is capped to the terminal width (no wider than the
  terminal).
- The visible height is derived from the terminal height (no fallback
  to a larger size).
- Rendering does not panic at 25×8.

On growth (resize to a larger size), the overlay restores the normal
layout with a border.

## Binding table

The key-binding list is defined once as data — a single source of truth
that the help renderer consumes and [Issue #34's documentation
test](#) will iterate. The `KeyBinding` struct has a `Key` (the key or
key sequence) and a `Description` (one-line description). `KeyBindings()`
returns the slice covering:

- Navigation: `n` (next match), `p` (previous match)
- Scrolling: `up`/`down` (one row), `u`/`d` (half page), `PgUp`/`PgDn`
  (full page)
- Panning: `,`/`.` (one column), `<`/`>` (ten columns), `[`/`]` (half
  text width)
- Wrap: `w` (toggle wrap mode)
- Colour: `c` (toggle colour scheme)
- List toggle: `left`/`right`, `Tab`/`Shift+Tab` (hide/show file list)
- Reload: `r` (reload current file)
- Help: `h`/`?` (open this help)
- Quit/cancel: `q` (quit), `ctrl+c` (cancel and exit 130), `Esc`
  (dismiss overlay)

## Footer slot

`HelpFooter()` returns the footer text displayed at the bottom of the
help overlay. Issue #34 fills the footer slot with the scale,
record-limit, and memory statements from the PRD's *Resources and
responsiveness* section. The footer is the shared `scaleLimitsText`
constant — the same text that appears in the README's scale section —
so neither sink can drift from the other. The footer is fixed
app-authored text (no runtime-string substitution points), so it needs
no sanitization; the Issue #6 sink-safety row verifies the rendered
output is safe. When non-empty, the footer is appended after a blank
line. See [documentation-sync](documentation-sync.md).

## Sink safety

The help overlay is fixed app-authored text (the binding table and
footer). It passes the [Issue #6 sink-safety](safe-presentation.md)
check: in the no-style composition path, no dangerous control bytes
survive in the output. With styles enabled, no fixture payload appears
after an unescaped ESC. Any substituted text (such as the Issue #34
footer) is routed through the Issue #6 utility.

## Cross-references

- [Issue #31](../issues/031-help-overlay.md)
- [PRD: Colours, overlays, and key precedence](../PRD-vrg.md)
- [Overlay precedence (Issue #32)](overlay-precedence.md)
- [Outcome contract (Issue #9 error overlay)](outcome-contract.md)
- [Browse tracer (Issue #15 pop-up)](browse-tracer.md)
- [Safe presentation (Issue #6)](safe-presentation.md)
- [Theme module (Issue #7)](theme-module.md)
