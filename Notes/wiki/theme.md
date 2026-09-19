# Theme — `c` colour toggle, inverse matches, underlines (Issue #7)

Delivered by
[Issue #7](../issues/007-theme-colour-toggle-and-match-styles.md):
`internal/theme` grows from the Issue #5 seam (`Inverse`/`Underline`
decorators on `Dark()`/`Plain()`) into the full style set the PRD's
Module Design assigns it — base, gutter, match, current-match,
indicator, overlay, filename-rule, and file-list styles — plus the `c`
toggle. Relevant PRD sections: *Colours, overlays, and key precedence*
(first bullet) and *Module Design → Theme* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). See [browse-tracer.md](browse-tracer.md)
for the Issue #5 seam this completes.

## Schemes and the `c` toggle

A `theme.Theme` is the active scheme's style set; `fg`/`bg` hold the
scheme's base colours as SGR colour parameters.

- `theme.Dark()` — white on black (`37;40`), the initially active
  scheme; the model's `theme` field starts there.
- `theme.Light()` — black on white (`30;47`).
- `Theme.Toggled()` — a pure value transform dark↔light; the App's
  base-state `c` keypress assigns `m.theme = m.theme.Toggled()`. There
  is no persistence: the scheme lives only in the model for the
  session.
- `theme.Plain()` — the no-style composition path unchanged since
  Issue #5: every decorator is the identity so the sink-safety
  raw-output assertions keep working; `Toggled()` on it is a no-op.

## The style set

Each decorator wraps a run in an SGR sequence and closes by
re-asserting the base colours **and clearing underline**
(`\x1b[<fg>;<bg>;24m`), so styled runs compose inside a `Base`-styled
frame without leaking attributes into the cells that follow. `Base`
itself is the frame's outermost wrap, ending in a plain `\x1b[0m`, so
even padding cells carry the background colour.

- **True-inverse matches** — `Match` renders the scheme's colours
  swapped (SGR 3x↔4x by ±10): a dark-scheme match is literally black on
  white, a light-scheme match white on black — each equal to the other
  scheme's base pair. This is a computed colour swap, not SGR 7
  reverse video, so the inverse tracks the scheme rather than the
  terminal's defaults.
- **Current-match underline** — `CurrentMatch` is the inverse pair
  plus `;4`. Until [Issue #13](../issues/) lands navigation, the
  "current matched line" is the current file's first stop.
- **Indicator** — `Indicator` is the same inverse style as `Match`;
  the hidden-content `_`/`*` markers of Issues #20 will consume it.
- **Gutter, filename rule, file list** — `Gutter`, `FilenameRule`,
  `FileList` all render the base colours.
- **Current file** — `CurrentFile` is the base colours plus `;4`: the
  current file-list entry is underlined in both schemes.
- **Overlay** — `Overlay(rows)` frames interior rows in a plain
  single-line border (`┌─┐ │ └─┘`), padding short rows to the widest,
  all in the base colours; on the no-style path the border still draws
  because it is structure, not styling. Interior width is measured in
  terminal cells by the shared ANSI-aware
  `safepresentation.CellWidth` — [Issue #39](unified-rendering.md)
  replaced the rune-per-cell `cellWidth`, so borders align for wide
  and combining interior text.
  The error overlay (#9), help overlay (#31), and file-change pop-up
  (#15) consume it.

## How rendering consumes the theme

`View()` wraps the whole composed frame — searching screen or browse —
in `theme.Base`, so the `c` toggle repaints every cell's colours on the
next render. Inside the frame, `browseView` computes the current
matched line (the first stop of `idx.Files[m.cur]`), and each region
styles itself: `listCell` uses `FileList`/`CurrentFile`, the gutter
`Gutter`, `filenameRule` `FilenameRule`, and `contentText` wraps each
maximal highlighted run in `CurrentMatch` or `Match` depending on
whether the line is the current matched line. No ANSI sequence is
written outside `internal/theme` — the styling-tui rule against
scattered escapes holds.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/theme/theme_test.go` (external package) covers both schemes'
colour pairs, the pure `Toggled` flip, the true-inverse match pair in
each scheme, the current-match underline, the inverse indicator, the
underlined current file versus plain file-list entry, the bordered
base-colour overlay, and the `Plain` no-style path;
`internal/app/browse_test.go`'s `TestCTogglesColourScheme` drives `c`
through `Update` and asserts the composed `View()` flips
white-on-black ↔ black-on-white and back, with the current-line match
inverse and underlined in both schemes.
