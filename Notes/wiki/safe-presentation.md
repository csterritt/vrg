# Safe presentation — the shared all-sink utility

Issue #6 (`Notes/issues/006-safe-presentation-utility-for-all-sinks.md`,
tasks `Notes/tasks/006-safe-presentation-utility-for-all-sinks.md`)
generalized the Issue #5 escaping core and the minimal Issue #1
`cli.Escape` escaper into one shared package, `internal/present`, that
every output sink routes through — so raw control sequences from
searched data can never reach the terminal as instructions.

PRD cross-references: "Text, graphemes, and safe presentation" (the
sanitization bullets) and "Module Design" in `Notes/PRD-vrg.md`, which
assigns the shared utility to a package rather than a top-level module.

## Canonical contracts per sink class

**Paths and filenames** — `present.Path(raw []byte) string` renders raw
path bytes as a single safe display line. `\n`, `\r`, `\t` become the
two-character forms `\n`, `\r`, `\t`; a literal backslash doubles to
`\\`; invalid UTF-8 bytes become `\xNN`; other C0 controls take caret
notation (`^[` for ESC) and DEL takes `^?`; C1 controls take `\uXXXX`;
valid printable Unicode passes through. This is the single-line
filename form embedded anywhere a name is displayed — the file list,
the filename rule, usage-error diagnostics, and (later) pop-ups,
overlays, and help substitutions. The raw path bytes — never the
display form — remain the key for identity, ordering, and file access.

**File content** — `present.LineOf(raw []byte) Line` presents one source
line, terminator included, as display cells. Invalid UTF-8 renders as
U+FFFD while its raw-byte mapping is retained; C0 controls and DEL use
caret notation; C1 controls use `\uXXXX`; LF and CRLF are structural
line terminators, never displayed (their bytes map to the end-of-line
position); a standalone CR renders as `^M`; a tab expands with space
cells to the next multiple of eight source-display columns as one
cluster (Issue #16 replaced the provisional single-cell `→`). Grapheme
clusters go through `x/ansi` width accounting and carry `Lead`/`Cont`
cell marks — `Lead` on a cluster's first cell, `Cont` on a multi-cell
unit's trailing cells — the shared segmentation/width policy the
Issue #16 row model wraps on (see [wrap-mode.md](wrap-mode.md)); a
cluster mixing printable and dangerous forms falls back to per-rune
rules so no control byte survives verbatim. A zero-width unit joins
the previous cell's text — a combining mark extends its base — except
at line start, where it takes a provisional `Lead` cell of its own on
a `◌` (U+25CC) dotted-circle base so a standalone cluster is always
one visible cell — a bare mark would merge into the previous terminal
cell and paint nothing (the Issue #21
fallback; see
[grapheme-highlight-expansion.md](grapheme-highlight-expansion.md)).
Every emitted unit records
per-byte `lo`/`hi` cell maps; `Line.Span(start, end)` maps a raw byte
range to its display `Span` — interior bytes expand to their whole
unit, a match covering an ESC byte highlights both `^[` cells, and a
range covering only removed terminator bytes or a zero-width position
yields a marker span (`Start == End`). Issue #21 adds a third cell
mark, `Blank`, never produced here: the row and clip layers mark the
filler cells they substitute for split clusters so a covering
highlight never styles them. Issue #22 adds
`LineOfBOM` — the same escaper run over a hidden prefix: a file's
first line keeps its leading UTF-8 BOM in `Raw` while the three bytes
paint nothing and map to the line-start position, the raw-file half of
the coordinate split described in
[line-terminators-and-bom.md](line-terminators-and-bom.md).

**Diagnostics** — `present.Diagnostic(s string) string` renders
diagnostic text while preserving the message's own line structure: LF
is a real line boundary and CRLF counts as one boundary; a standalone
CR takes `^M`; tabs expand to the next multiple of eight display
columns (counting escaped forms and rune widths); other C0 controls and
DEL take caret notation; C1 takes `\uXXXX`; invalid UTF-8 takes `\xNN`.
Printable text — including backslash — passes through, so a filename
embedded via `Path` keeps its single-line escaped form: **escape the
filename first, then embed it**, and its newline can never become a
diagnostic paragraph break.

## Sink wiring and the Issue #1 escaper replacement

`cli.Escape` is gone. `internal/cli` now escapes every hostile
substitution — unsupported option tokens, the excess operand, the root
operand — through `present.Path`; `cmd/vrg` renders its boundary error
text (`vrg: <err>`) through `present.Diagnostic`. The browse sinks call
`present.Path` for list entries and the filename rule and consume
`present.Line`/`Cell`/`Span` for panel content. Generated command-line
help on stdout contains only fixed text; its sink-safety row guards
that property rather than a substitution path.

## The shared sink-safety table

`internal/app/sinksafety_test.go` holds the Issue #5 hostile fixture
set restructured as a shared, extensible table:

- **Fixtures** (`hostileFixtures`): OSC `\x1b]0;pwned\x07`, CSI
  `\x1b[2J`, C0 controls (`\x07 \x08 \x1b`), C1 `\xc2\x85`, DEL, a
  standalone CR, invalid UTF-8 path bytes, and an embedded filename
  newline. Each carries the bytes to inject, the raw bytes forbidden
  verbatim in output, the distinctive post-ESC payload, and the
  independently written escaped forms a path-bearing, content, or
  diagnostic sink must show (`wantPath`/`wantText`/`wantDiag`).
- **Sink rows** (`sinkSafetySinks`): file-list entry, filename rule,
  panel content, usage-error stderr (the `cli.Parse` diagnostic plus
  `cli.HelpText()` composition `run` writes), CLI-help stdout —
  the generated command-line help is its own sink, distinct from the
  Issue #31 TUI help dialog that adds its own row later — and, since
  Issue #9, the **error overlay**: the fixture bytes ride in as captured
  child stderr and the check asserts the `Diagnostic`-escaped
  `wantDiag` forms inside the border — and, since Issue #11, the
  **stderr replay** row: the replay writer is driven through
  `renderReplaySink` and the check asserts the escaped `wantDiag` and
  `wantPath` (embedded filename) forms in the replayed output — and,
  since Issue #15, the **file-change pop-up**: `renderPopupSink` names
  the navigation destination with the fixture bytes and the check
  asserts the `Path`-escaped `wantPath` form inside the box — and,
  since Issue #31, the **help overlay**: `renderHelpSink` injects the
  fixture bytes through the help overlay's footer substitution slot
  and the check asserts the `Diagnostic`-escaped `wantDiag` forms
  inside the border — and, since Issue #34, the **help footer note**:
  `renderHelpFooterSink` substitutes the fixture at every
  runtime-substitution point of the rendered footer (each installed
  `limitNotes` entry plus a dedicated injection), scrolls the overlay
  to the footer's rows, and the check asserts the escaped `wantDiag`
  forms and the real note's `64 MiB` text inside the border (see
  [documentation.md](documentation.md)).
- **Method**: each fixture × sink renders through the real composition
  path under `theme.Plain` — the no-style path where no escape byte may
  legitimately appear — and asserts on the **raw output before any ANSI
  stripping** that no fixture control byte survives and that none of
  the universal set (`\x1b`, `\x07`, `\x9b`, `\xc2\x85`, bare `\r`)
  appears at all; TUI rows also pin the frame at height-1 newlines. A
  styled pass then asserts the fixture's distinctive payload never
  appears immediately after an unescaped ESC. (Stripping ANSI and
  comparing would erase the very evidence being sought.)

**Later-sink ownership**: each issue that introduces a new sink routes
it through `internal/present` and adds a `sinkSafetySinks` row without
duplicating fixtures — Issue #9 (error overlay — delivered; see
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)),
Issue #11 (stderr replay — delivered; see
[stderr-replay.md](stderr-replay.md)), Issue #15 (file-change pop-up —
delivered; see
[file-change-popup.md](file-change-popup.md)),
Issue #31 (TUI help dialog substitutions — delivered; see
[help-overlay.md](help-overlay.md)), Issue #34 (the rendered help
footer note — delivered; see
[documentation.md](documentation.md)).

## Regression surface

The Issue #5 core cases re-ran unchanged against the generalized
utility — they moved with it to `internal/present` (`Path`, `LineOf`,
`Line`, `Cell`, `Span`) with only mechanical renames. The Issue #1 CLI
output tests — the `TestGeneratedHelpStdout` and `TestCLIOutputSafety`
named groups in `cmd/vrg/main_test.go` and the `internal/cli` suites —
pass unmodified against the replacement.

## Files

- `internal/present/present.go` — `Path`, `Diagnostic`.
- `internal/present/line.go` — `Line` (raw bytes + text + cells +
  `lo`/`hi` maps), `LineOf`, `Cell`, `Span`.
- `internal/present/doc.go` — package contract.

See also: [browse-tracer.md](browse-tracer.md) (the first sinks),
[cli-foundation.md](cli-foundation.md) (the escaper this replaces),
[unit-tests.md](unit-tests.md) (the table and test catalog).
