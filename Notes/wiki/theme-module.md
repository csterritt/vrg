# Theme module (Issue #7)

The active colour scheme and visual styles for the browse view,
delivered by [Issue #7](../tasks/007-theme-colour-toggle-and-match-styles.md).
Relevant PRD sections: *Colours, overlays, and key precedence* and
*Module Design → Theme*.

## Schemes

`internal/theme/theme.go` owns two colour schemes:

| Scheme | Foreground | Background | ANSI sequence |
|---|---|---|---|
| Dark (initial) | white | black | `\x1b[37;40m` |
| Light | black | white | `\x1b[30;47m` |

`New()` returns a theme with the dark scheme active. `Scheme()` reports
the active scheme. `Toggle()` flips between dark and light with no
persistence: a fresh `New()` is always dark regardless of prior
toggles on other instances. The no-style theme (`NoStyle()`) is
unchanged by `Toggle()`.

## Style set

The Theme supplies the full PRD style set:

| Method | Style | ANSI (dark) | Restores |
|---|---|---|---|
| `Base(s)` | base colour pair | `\x1b[37;40m…\x1b[0m` | reset |
| `Gutter(s)` | base colours | `\x1b[37;40m…\x1b[0m` | reset |
| `Match(s)` | true inverse of base | `\x1b[30;47m…\x1b[37;40m` | base |
| `CurrentMatch(s)` | true inverse + underline | `\x1b[30;47;4m…\x1b[37;40m` | base |
| `Indicator(s)` | inverse (same as Match) | `\x1b[30;47m…\x1b[37;40m` | base |
| `Underline(s)` | underline | `\x1b[4m…\x1b[37;40m` | base |
| `FileList(s)` | base colours | `\x1b[37;40m…\x1b[0m` | reset |
| `FilenameRule(name)` | base + horizontal rule | `\x1b[37;40m── name ──\x1b[0m` | reset |
| `Overlay(s)` | base + single-line border | `\x1b[37;40m┌…┐│ … │└…┘\x1b[0m` | reset |

### True inverse matches

Matches use the **true inverse** of the active scheme's base colours,
not the SGR 7 reverse-video attribute. In the dark scheme (white on
black), matches are black on white (`\x1b[30;47m`). In the light
scheme (black on white), matches are white on black (`\x1b[37;40m`).
This ensures predictable colours across terminals with custom
palettes.

`Match` and `Indicator` restore the base colour pair after the styled
span (not a full reset), so text after a match remains in base colours
when rendered within a `Base` wrapper.

### Current-match underline

`CurrentMatch` adds the SGR 4 underline attribute to the true-inverse
match colours: `\x1b[30;47;4m` (dark) or `\x1b[37;40;4m` (light). The
current matched line is the first stop for the current file (the first
stop until [Issue #13](../issues/013-match-navigation.md) adds
navigation). Non-current matched lines use `Match` (no underline).

### Current-file underline

`Underline` wraps a string in the SGR 4 underline sequence and restores
the base colours. The browse view applies it to the current file-list
entry in both schemes.

### Overlay style

`Overlay` wraps content with a plain single-line border using
box-drawing characters (`┌┐└┘─│`) and the active scheme's base colours.
The no-style theme returns the content without ANSI sequences or a
border.

## No-style theme

`NoStyle()` disables all ANSI sequences. Every style method returns
its input unchanged (or the plain formatting for `FilenameRule`).
This is the sink-safety testing path: rendering through the no-style
theme produces no ANSI escape sequences, so any control byte in the
output must come from unsanitized external data. See
[safe-presentation](safe-presentation.md).

## How rendering consumes the theme

The browse view ([browse-tracer](browse-tracer.md)) routes rendering
through the theme:

- Each composed line is wrapped in `theme.Base()` for the active
  scheme's base colours.
- The current file-list entry is wrapped in `theme.Underline()`.
- Matched spans use `theme.Match()` (non-current lines) or
  `theme.CurrentMatch()` (current matched line), determined by
  comparing the line number to the first stop's line number for the
  current file.
- The filename rule and gutter are plain text within the `Base`
  wrapper (base colours applied by the outer wrapper).

The `c` keypress in the browse state calls `theme.Toggle()` on the
model's theme, flipping the composed `View()` styling between dark and
light with no persistence.

## Issue #5 to Issue #7 changes

Issue #5 landed a minimal theme seam with `Underline(s)` (SGR 4) and
`Reverse(s)` (SGR 7 reverse video). Issue #7 replaces `Reverse` with
explicit true-inverse colour pairs (`Match`/`CurrentMatch`), adds the
scheme toggle, and expands the style set to include `Base`, `Gutter`,
`Indicator`, `FileList`, `FilenameRule`, and `Overlay`.

## Testing

See [unit-tests](unit-tests.md) for the theme unit test and app
toggle test catalogs.
