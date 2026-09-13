# Safe presentation (Issue #6)

The shared safe-presentation utility that sanitizes all external data
before it reaches any output sink. Delivered by
[Issue #6](../issues/006-safe-presentation-utility-for-all-sinks.md),
generalizing the Issue #5 path/content core and unifying it with the
Issue #1 `cli.Escape` escaper. Relevant PRD section: *Text, graphemes,
and safe presentation*.

The terminal-safety contract: external content, filenames,
diagnostics, help, and replayed stderr must never emit control
sequences that terminal emulators interpret as commands. Sanitization
does not promise elimination of every Unicode visual confusable; its
contract is that raw control sequences from external data never
execute.

## Location and API

`internal/safepresentation` is the canonical source for all escaping.
Exports:

- `EscapePath(raw []byte) PathDisplay` — single-line path escaping
  (Issue #5; now also the backing for `cli.Escape`).
- `EscapeContent(raw []byte) ContentDisplay` — multi-line content
  escaping with byte→cell mappings (Issue #5).
- `EscapeDiagnostic(raw []byte) string` — multi-line diagnostic
  escaping that preserves line boundaries (Issue #6).

`internal/sinkfixtures` holds the shared hostile-fixture set and
sink-safety assertion helpers. Every sink row iterates over the same
fixtures; later issues add rows without duplicating fixtures.

## Path contract (single-line)

`EscapePath` escapes raw path bytes for safe single-line display:

- `\n`, `\r`, `\t` → `\n`, `\r`, `\t` (backslash escapes)
- literal `\` → `\\`
- invalid UTF-8 bytes → `\xNN`
- other C0 controls → caret notation (`^X`)
- DEL (0x7f) → `^?`
- C1 controls (U+0080–U+009F) → `\u00XX`
- valid printable Unicode → preserved

`PathDisplay.ByteCells[i]` records the `[start, end)` display cell
range occupied by original byte `i`, so highlight rendering can map
byte ranges to cells. Displayed strings never become filesystem keys;
callers retain original bytes for identity, ordering, and file access.

## Content contract (multi-line)

`EscapeContent` escapes raw content bytes for safe display:

- invalid UTF-8 → U+FFFD (one cell), raw-byte mapping retained
- C0 controls and DEL → caret notation (`^[` for ESC, `^?` for DEL)
- C1 controls → `\u00XX`-style escapes
- LF and CRLF → line terminators, never displayed; bytes map to
  end-of-line position
- standalone CR (not followed by LF) → `^M`
- tab → single `→` placeholder cell (provisional pending Issue #16's
  eight-column-stop expansion)
- valid printable Unicode → preserved

`ContentDisplay.ByteCells[i]` records the `[start, end)` display cell
range for each original byte. A match covering an ESC byte highlights
both cells of `^[`.

## Diagnostic contract (multi-line, line-preserving)

`EscapeDiagnostic` escapes raw diagnostic bytes for safe display while
preserving real line boundaries:

- LF → preserved as a line boundary (not escaped)
- CRLF → normalized to LF (CR consumed, LF retained; no raw CR survives)
- tab → expanded to eight-column stops (column counter resets at each
  newline)
- other C0 controls and DEL → caret notation (`^[` for ESC, `^?` for
  DEL)
- C1 controls → `\u00XX` escapes
- invalid UTF-8 bytes → `\xNN` escapes
- backslash → not escaped (so embedded filenames escaped through
  `EscapePath` are not double-escaped)
- valid printable Unicode → preserved

### Embedded filename single-line rule

A filename embedded by vrg in a diagnostic must first be escaped
through `EscapePath` (making it single-line), then the whole diagnostic
is escaped through `EscapeDiagnostic`. The composition works because
`EscapePath` turns `file\nname` into the literal `file\nname` (with a
backslash-n, not a real newline), and `EscapeDiagnostic` does not
escape backslashes, so the escaped filename passes through unchanged.
Filename newlines cannot become diagnostic paragraph breaks.

`app.EscapePathForDiagnostic(raw string) string` is the helper for
callers that need to embed a filename in a diagnostic. It delegates to
`safepresentation.EscapePath`.

## Sink wiring

Every current sink routes through the shared utility:

| Sink | Escaper | Location |
| --- | --- | --- |
| File-list entry | `EscapePath` | `internal/app` renderBrowse |
| Filename rule | `EscapePath` | `internal/app` renderBrowse |
| Panel content | `EscapeContent` | `internal/app` renderLineWithHighlights |
| Usage-error stderr | `EscapePath` via `cli.Escape` | `internal/cli` diagnostic; `cmd/vrg` stderr |
| Generated CLI-help stdout | fixed text (no external data) | `internal/cli` renderHelp |
| Replayed stderr diagnostic (Issue #11) | `EscapeDiagnostic` via `sanitizeDiagnostic` | `internal/app` collectDiagnostic; `cmd/vrg` replay loop |

The Issue #1 `cli.Escape` is now a one-line wrapper around
`safepresentation.EscapePath`. The duplicated escaper implementation
in `internal/cli` was removed.

The `internal/app` `sanitizeDiagnostic` function (which only mapped
ESC and C1 CSI to spaces) was replaced with a delegate to
`EscapeDiagnostic`, satisfying the full diagnostic contract: line
preservation, tab expansion, and complete control escaping.

## Shared sink-safety table

`internal/sinkfixtures` provides:

- `Fixtures` — the shared hostile-fixture slice. Covers OSC, CSI, C0,
  C1, DEL, standalone CR, invalid UTF-8, and embedded filename newline.
  Each fixture has a `Name` (for subtest labels), `Raw` (the hostile
  bytes), and `Payload` (the distinctive bytes that must never appear
  immediately after an unescaped ESC in styled output).
- `NoControlBytes(s string) bool` — asserts no C0 controls or DEL
  survive (excluding newlines from the multi-line layout). For sinks
  that incorporate external data.
- `NoDangerousControls(s string) bool` — asserts no dangerous C0
  controls or DEL survive (excluding newlines and tabs which are
  legitimate formatting in fixed-text sinks like generated help).
- `NoPayloadAfterESC(s string, payload []byte) bool` — the styled
  assertion: with styles enabled, ANSI ESCs are legitimate, but the
  fixture's ESC should be escaped as `^[`. If the fixture's payload
  appears after a raw ESC, the ESC was not escaped.

### No-style composition method

The no-style path renders through `theme.NoStyle()`, which disables all
ANSI sequences. Any control byte in the output must come from
unsanitized external data. The raw output is checked before any ANSI
stripping.

### Styled assertion method

With styles enabled (`theme.New()`), ANSI ESCs are legitimate (from
the Issue #7 style set: `Base`, `Match`, `CurrentMatch`, `Indicator`,
`Underline`, `Gutter`, `FileList`, `FilenameRule`, `Overlay`). The
styled assertion checks that the fixture's distinctive payload never
appears immediately after an unescaped ESC — if it did, the fixture's
ESC was not escaped and the payload could form a dangerous terminal
sequence.

### Future sink ownership

Later issues add sink rows by iterating over the same `sinkfixtures.Fixtures`
slice. A new sink adds a test that calls the sink with each fixture and
asserts `NoControlBytes` (no-style) and `NoPayloadAfterESC` (styled).
No fixture duplication is needed. Issue #11 added the replayed stderr
diagnostic sink row. Issues #15, #31, and #34 will add their own rows.

## Tests

See [unit-tests](unit-tests.md) for the safe-presentation, sink-safety
table, and diagnostic test catalogs.

## References

- Issue #6: `Notes/issues/006-safe-presentation-utility-for-all-sinks.md`
- PRD: `Notes/PRD-vrg.md` (Text, graphemes, and safe presentation)
- Issue #5 browse tracer: [browse-tracer](browse-tracer.md)
- Issue #1 CLI foundation: [cli-foundation](cli-foundation.md)
