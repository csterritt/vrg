# Theme — colour schemes, `c` toggle, and the style set

Issue #7 (`Notes/issues/007-theme-colour-toggle-and-match-styles.md`,
tasks `Notes/tasks/007-theme-colour-toggle-and-match-styles.md`)
turned the minimal Issue #5 seam in `internal/theme` into the real
Theme module: two colour schemes, an in-memory toggle key, and the
named style set every rendered element draws through.

PRD cross-references: "Colours, overlays, and key precedence" (first
bullet) and "Module Design → Theme" in `Notes/PRD-vrg.md`.

## Schemes and the `c` toggle

A scheme is a foreground/background SGR pair:

- **Dark** — white on black (`37;40`), initially active.
- **Light** — black on white (`30;47`).

`theme.Dark()` constructs the initial state; `Theme.Toggle` returns the
Theme with the other scheme active. The toggle is in-memory state only —
nothing is persisted. `Theme.Light` reports which scheme is active.

`Model` handles `c` in `Update` only in `phaseBrowse` (during searching,
ordinary keys stay inert): `m.theme = m.theme.Toggle()`. `View` wraps
each composed frame in `theme.Base`, which emits the base pair at the
frame's head and a full reset at its end — so the frame's first SGR
sequence identifies the active scheme. `renderBrowse` pads every row to
the terminal width so the base background covers the whole screen.

## The style set

Every style is a method on `Theme`; all are the identity under
`theme.Plain`, the no-style composition path the sink-safety tests
render through (see
[browse-tracer.md](browse-tracer.md) and
[safe-presentation.md](safe-presentation.md)). Styled runs emit explicit
colour pairs — not bare SGR 7 — so each style is correct independent of
ambient state, and each restores the scheme's base pair afterwards so
nested styles compose inside a `Base`-painted frame.

- `Base` — the scheme's pair; wraps each composed frame.
- `Gutter`, `FileList`, `FilenameRule` — the base colours, applied to
  the line-number gutter, each file-list entry, and the filename rule.
- `Match` — the **true inverse** of the active scheme's base pair:
  `scheme.inverse()` exchanges foreground and background (SGR colour n
  foreground ↔ n+10 background), so a dark-scheme match is black on
  white and a light-scheme match is white on black.
- `CurrentMatch` — `Match` plus underline: inverse pair and SGR 4 on,
  underline and the base pair restored after. Renders matches on the
  current matched line (the matched-line cursor's selected stop since
  Issue #13; see [match-navigation.md](match-navigation.md)).
- `Indicator` — the inverse pair, for the hidden-content `_`/`*`
  markers (consumed by Issue #20's gutter and right-column indicators).
- `CurrentFile` — underline only; the ambient base colours are
  untouched. Renders the current file-list entry in both schemes.
- `Overlay` — frames content lines in a plain single-line border
  (`┌─┐`/`│`/`└─┘`, each line padded to the widest) painted in the base
  colours; consumed by the overlay-owning issues (#9, #15, #31).

## How rendering consumes the theme

In `internal/app/browse.go`:

- Each file-list entry goes through `FileList`, or `CurrentFile` for the
  current file — the underline survives list-width padding because the
  padding is written outside the styled run.
- `filenameRule` output passes through `FilenameRule`.
- `contentRow` wraps the right-justified number and its two spaces in
  `Gutter`, and passes the `present.Cell`s plus a `current` flag —
  whether the row's source line is the cursor stop's line — into
  `renderCells`, which chooses `CurrentMatch` or `Match` per run,
  including the end-of-line marker space.

## Tests

`internal/theme/theme_test.go` pins every contract at the style level:
both schemes' pairs, the initially-dark state, the toggle round trip,
true-inverse matches and indicators in each scheme, the current-match
and current-file underlines, the overlay's base colours and single-line
border, and `Plain` identity. In `internal/app`, the Issue #5 SGR
assertions moved to the explicit-pair codes, `TestColourToggleFlipsView-
Styling` drives `c` through `Update` and asserts the frame's first SGR
sequence flips `37;40` → `30;47` → `37;40`, and
`TestCurrentLineMatchUnderlined` distinguishes the underlined
current-line match from the plain-inverse match on another matched
line.

## Files

- `internal/theme/theme.go`, `internal/theme/theme_test.go` — schemes,
  `Toggle`/`Light`, the style set, `Plain`, and their tests.
- `internal/app/app.go` — `c` handling and the `Base` frame wrap.
- `internal/app/browse.go` — style consumption, the `current` flag, and
  full-width row padding.

See also: [browse-tracer.md](browse-tracer.md) (the composition these
styles paint), [safe-presentation.md](safe-presentation.md) (the
`Plain` path's role in the sink-safety table), and
[cancellation-and-cleanup.md](cancellation-and-cleanup.md) (key
precedence around `ctrl+c`).
