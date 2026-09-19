# Safe-presentation utility for every output sink (Issue #6)

Delivered by
[Issue #6](../issues/006-safe-presentation-utility-for-all-sinks.md):
the Issue #5 escaping core is generalized into the single shared
utility that every output sink routes through, the minimal Issue #1
`cli.Escape` escaper is replaced, diagnostics gain their own
presentation rules, and the Issue #5 hostile fixture set is
restructured into the shared, extensible sink-safety table. Relevant
PRD section: *Text, graphemes, and safe presentation* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). The terminal-safety contract stays
narrow: raw control sequences from external data never execute, and
original bytes remain the key for identity, ordering, and file access —
displayed strings never become filesystem keys.

## The shared utility (`internal/safepresentation`)

Three entry points cover the sink classes:

- **Paths** — `EscapePath([]byte) string` is the canonical single-line
  path/filename contract (unchanged from
  [Issue #5](browse-tracer.md)): `\n`, `\r`, `\t` become the
  two-character escapes; a literal backslash doubles; other C0 controls
  and DEL use caret notation (ESC is `^[`, DEL is `^?`); C1 controls and
  non-printable runes use `\uXXXX`; invalid UTF-8 bytes use `\xNN`;
  valid printable Unicode is preserved.
- **File content** — `MapContent(raw []byte) Mapped`: line splitting
  owns LF/CRLF terminators; a standalone CR renders `^M`; other C0
  controls and DEL use caret notation; C1 and non-printable runes use
  `\uXXXX`; invalid UTF-8 bytes render U+FFFD with retained byte
  mappings; a tab expands with blank cells to the next multiple of
  eight source-display columns — the structural eight-column-stop rule
  [Issue #16](wrap-mode.md) landed, replacing the provisional `→`.
  `Mapped.CellsCovering` maps source byte ranges onto the cells of
  escaped forms so highlights cover every cell an escape produced;
  FileBuffer's `Line.CellsCovering` layers the rg-line coordinate
  translation and terminator-to-EOL rules on top
  ([Issue #22](line-structure.md)).
  `Mapped.Clusters` (Issue #16) segments the cells into grapheme-cluster
  boundaries — the shared segmentation and cell-width policy:
  each escaped-form character and each invalid-byte replacement is a
  single-cell cluster, while a wide glyph or a tab expansion is one
  unbreakable multi-cell cluster, so wrapping never splits a cluster.
  Issue #21
  ([grapheme-highlight-expansion.md](grapheme-highlight-expansion.md))
  consumes the same records for highlight expansion — a partial-cluster
  match covers its whole cluster — and for the standalone combining
  cluster's Issue #43 `◌`-plus-marks one-cell fallback, so a highlight
  is never an inaccessible zero-cell span.
- **Diagnostics** — `EscapeDiagnostic(string) string` (new): real
  diagnostic line boundaries are preserved — LF stays, CRLF normalizes
  to LF — tabs expand to the next multiple of eight display columns,
  other C0 controls and DEL use caret notation (standalone CR is `^M`),
  C1 and non-printable runes use `\uXXXX`, invalid UTF-8 bytes use
  `\xNN`, and a backslash is literal. Any external string embedded in a
  diagnostic — a filename, an error message — is escaped first with
  `EscapePath`, so its newline can never forge a diagnostic line
  boundary.

`internal/safepresentation/sinktest` is the test-support half: the
shared hostile fixture set and the raw-output assertions. Imported only
by `_test.go` files, it never reaches the binary.

## Replacing the Issue #1 escaper

`cli.Escape` is gone; every former call site now uses the shared
utility:

- `internal/cli` usage diagnostics embed operands single-line-escaped
  via `EscapePath`, and `usageErrorf` passes the composed diagnostic
  through `EscapeDiagnostic`.
- `renderHelp` output passes through `EscapeDiagnostic`, so the
  generated help's layout tabs expand to fixed columns — the stdout
  help sink is routed through the utility like every other. This is the
  Issue #1 command-line help, distinct from the Issue #31 TUI help
  dialog, which adds its own sink-safety row through `helpFixtureView`.
- `internal/app` and `cmd/vrg` stderr diagnostics (start failure, the
  post-restoration `writeFailureDiag`, the working-directory error)
  escape embedded error text via `EscapePath` — external data
  single-lined, exactly the Issue #1 semantics the replaced escaper
  provided.

## The sink-safety table

The Issue #5 hostile fixture set is restructured into
`sinktest.Fixtures` — OSC (`\x1b]0;pwned\x07`), CSI (`\x1b[2J`), a C0
run (`\x07\x08\x1b`), C1 (`\xc2\x85`), DEL, standalone CR, invalid UTF-8
bytes, and an embedded filename newline — plus two shared assertions:

- `AssertRawOutput`: the sink's raw output, before any ANSI stripping,
  is valid UTF-8 carrying no C0 control other than the `\n` framing, no
  DEL, and no C1. Sinks render through the **no-style composition path**
  (`theme.Plain()`), so no escape byte may legitimately appear and any
  control byte present can only be a fixture byte that survived.
- `AssertPayloadNotEscaped`: under the styled theme, the fixture's
  distinctive payload (e.g. `]0;pwned`) never appears immediately after
  an unescaped ESC — legitimate style sequences may emit ESC, fixture
  bytes may not complete a sequence.

`sinktest.Run(t, sinks)` drives every fixture through every `Sink` row
as `<sink>/<fixture>` subtests; each row's `Render` returns the raw
output of the real composition path and must fail when the fixture
never reached the sink (a silent drop cannot masquerade as a pass). The
current table lives in `internal/app/sinksafety_test.go` with ten
rows — file-list entry, filename rule, panel content, the Issue #9
error overlay, the Issue #15 file-change pop-up (see
[file-change-popup.md](file-change-popup.md)), the Issue #31 TUI help
dialog (`helpFixtureView` substitutes the fixture into the `helpFooter`
slot the renderer routes through `EscapeDiagnostic` — see
[help-overlay.md](help-overlay.md)), the Issue #34 help footer note
(`footerFixtureView` substitutes the fixture at the footer's
runtime-substitution point and asserts it ends the composed body — see
[documentation-limits.md](documentation-limits.md)), usage-error stderr,
CLI-help stdout, and the Issue #11
stderr replay (a load-failure diagnostic embedding the fixture as a
filename, collected and written through `replayDiags` — see
[stderr-replay.md](stderr-replay.md)).

**Later-sink ownership**: each issue that introduces a sink routes it
through this utility and extends the table — Issue #9 (error overlay),
Issue #11 (stderr replay), Issue #15 (pop-up), Issue #31 (TUI help
dialog), and Issue #34 (the rendered help footer and any other
generated text path accepting runtime strings) are landed — reusing
`sinktest.Fixtures` without duplicating them.

## Regressions

The Issue #5 core escaping tests (`safepresentation_test.go`), the
browse sink-safety test (`TestHostileFixtureRawOutput`), and the Issue
#1 CLI output tests (`TestGeneratedHelpStdout`,
`TestCLIOutputSafety`, and the `internal/cli` suites) all pass
unchanged against the generalized utility.

## Tests

See [unit-tests.md](unit-tests.md):
`internal/safepresentation/diagnostic_test.go` covers the diagnostic
rules (line boundaries, tab stops, caret/`\u`/`\xNN` forms, literal
backslash, single-lined embedded filenames, no raw controls);
`internal/app/sinksafety_test.go` hosts the ten-row sink-safety table
driven by `internal/safepresentation/sinktest`.
