# Documentation: scale, memory, and synchronization (Issue #34)

The README at the repository root is the single user-facing
documentation artifact. Issue #34 wrote the README, filled the
[Issue #31 help overlay footer](help-overlay.md) slot with the scale,
record-limit, and memory statements, and added documentation
synchronization tests that keep the README and footer honest against
the implementation. Relevant PRD sections: *Resources and
responsiveness*, *Out of Scope*, *Outcome and exit-status contract*.

## README

`README.md` at the repository root documents:

- **Invocation**: `vrg [flags] pattern [root]`, root defaults to `.`,
  `--` ends option parsing, combined short flags, cumulative `-u`.
- **Flags**: every allow-listed search flag and local help option from
  the [CLI's shared option declarations](cli-flag-forwarding.md), in
  short and long form with the exact no-argument spellings. The flag
  table is asserted against `cli.OptionDecls()` so it cannot drift from
  parsing.
- **Key bindings**: every binding from [Issue #31's binding
  table](help-overlay.md) (`app.KeyBindings()`), consumed from the same
  single data source as the help overlay renderer.
- **Exit status**: the complete table for 0, 1, 2, and 130, stating
  both reasons for exit 0 (successful search and browse, and
  command-line help), the flags-only missing-pattern usage error
  (`vrg -i` → exit 2), cancellation triggers (`ctrl+c` anywhere, `q`
  during search), and pre-TUI usage/root/start failures (exit 2). The
  search-derived values agree with the [Issue #9 outcome
  function](outcome-contract.md) (`app.DecideOutcome`).
- **Help**: bare `vrg` and `-h`/`--help` print command-line help to
  stdout and exit 0 without starting ripgrep or entering the TUI,
  distinct from the TUI's `h`/`?` help dialog.
- **Scale, record limits, and memory**: the three independent scale
  examples, the 64 MiB record limit with the base64 caveat, and the
  session-long memory-retention and no-cleanup-guarantee statements
  (see below).
- **Ripgrep**: ripgrep 15.x as the reference family; VRG supplies
  `--no-config` so ripgrep configuration files are never honoured.

## Scale examples

Three independent scale examples from the PRD's *Resources and
responsiveness* section:

- Approximately 10,000 matched files.
- Approximately 100,000 matched lines.
- Individual files around 50 MB.

These are **independent, not simultaneous capacity guarantees**.
Aggregate match data, record expansion, decoded buffers, and
visited-file retention determine memory demand.

The ~50 MB file example assumes UTF-8 (or near-UTF-8) content with
ordinary line lengths. Files with very long lines that ripgrep must
emit as base64 `bytes` (invalid UTF-8) expand by roughly a third in the
JSON stream; a single such line of tens of megabytes can produce a
`match` record over 64 MiB even when the file itself is under 50 MB.
Such matches are skipped and reported with the oversized-record
diagnostic (naming the path when recoverable); they are not browsable.

## 64 MiB record limit

The maximum JSON record payload is 64 MiB, excluding the newline
delimiter. Oversized records are consumed and discarded through the
next newline, counted separately, and parsing resumes. The
oversized-record diagnostic names the affected file path when the
`type` and `data.path` fields were parsed before the limit was reached
(the usual case). See [record-robustness](record-robustness.md) for the
full skip/count and integrity rules.

The 64 MiB limit is independent of source-file size: escaping or base64
can exceed it even for a source file under 50 MB. Oversized records use
the explicit skip/count and integrity rules, not silent truncation.

## Memory and termination limits

- Loaded buffers are retained for the session with no eviction.
- No aggregate memory bound is promised.
- No reliable OOM recovery is promised.
- No guaranteed terminal cleanup under forced termination.

Large searches or many visited files may exhaust memory and terminate
the process; graceful terminal cleanup cannot be guaranteed under
forced termination. See the PRD *Out of Scope* section: file-buffer
eviction, aggregate memory guarantees, and reliable recovery/cleanup
under OOM or OS-forced termination are explicitly out of scope.

## Help overlay footer

The [Issue #31 help overlay](help-overlay.md) has a footer slot that
Issue #34 fills with the scale, record-limit, and memory statements.
`app.HelpFooter()` returns the shared `scaleLimitsText` constant — the
same text that appears in the README's scale section — so neither sink
can drift from the other. The footer is fixed app-authored text (no
runtime-string substitution points), so it needs no sanitization; the
[Issue #6 sink-safety](safe-presentation.md) row verifies the rendered
output is safe.

## Synchronization tests

`internal/docs/docs_test.go` (package `docs_test`) contains the
documentation synchronization tests:

- `TestREADMEContainsEveryBinding` — iterates `app.KeyBindings()` and
  asserts each binding's key and description appear in the README.
- `TestREADMEContainsEveryAllowListedFlag` — iterates
  `cli.OptionDecls()` and asserts each search flag's short and long
  form appear in the README.
- `TestREADMEContainsLocalHelpOptions` — asserts the local help
  options (`-h`/`--help`) appear in the README.
- `TestREADMEFlagDocsFromSharedDeclarations` — asserts every declared
  option (help and search) appears in the README in short and long
  form with the exact no-argument spellings.
- `TestREADMEExitStatusAgreesWithOutcomeFunction` — calls
  `app.DecideOutcome` for exit 0, 1, 2, and record-loss-2 cases and
  asserts the README documents each exit status.
- `TestREADMEDocumentsCancellationTriggers` — asserts the README
  documents `ctrl+c` and exit 130 for cancellation.
- `TestREADMEDocumentsPreTUIFailures` — asserts the README documents
  usage, root validation, and start failures (exit 2).
- `TestREADMEDocumentsHelpOnlyPath` — asserts the README documents
  bare `vrg`, `-h`/`--help`, stdout, and the TUI help dialog
  distinction.
- `TestREADMEDocumentsFlagsOnlyUsageError` — asserts the README
  documents the pattern requirement (flags-only → exit 2).
- `TestREADMEDocumentsRipgrep15x` — asserts the README identifies
  ripgrep 15.x.
- `TestREADMEDocumentsNoConfig` — asserts the README states
  `--no-config` and the config file reference.
- `TestREADMEScaleExamples` — asserts the three scale examples and
  the independence qualification.
- `TestREADMERecordLimit` — asserts the 64 MiB limit, base64 caveat,
  oversized diagnostic, and recoverable path.
- `TestREADMEMemoryLimits` — asserts session retention, eviction,
  aggregate memory bound, OOM, and forced termination.
- `TestHelpFooterCarriesSameStatements` — asserts the help footer
  contains every scale/record-limit/memory token.
- `TestREADMEAndFooterBothCarryAllScaleLimitTokens` — asserts both
  the README and the footer contain every token, so deleting any
  statement from either sink fails the suite.

## Sink-safety row

`internal/app/help_footer_test.go` (package `app_test`) adds the
Issue #34 row to the [Issue #6 shared sink-safety
table](safe-presentation.md):

- `TestHelpFooterNonEmpty` — the footer is non-empty.
- `TestHelpFooterNoDangerousControls` — the footer text contains no
  dangerous control bytes.
- `TestHelpFooterSinkSafetyNoStyle` — the rendered help overlay with
  footer passes the no-style composition path: no dangerous control
  bytes in raw output.
- `TestHelpFooterSinkSafetyStyled` — with styles enabled, no fixture
  payload appears after an unescaped ESC.
- `TestHelpFooterRenderedInOverlay` — the rendered help overlay
  actually contains footer tokens (verified at 80×50 so the footer is
  visible).

The footer is fixed app-authored text with no runtime-string
substitution points, so the sink-safety row verifies the rendered
output is safe and guards against future runtime-string additions.

## Cross-references

- [Issue #34](../issues/034-documentation-scale-and-memory-limits.md)
- [PRD: Resources and responsiveness](../PRD-vrg.md)
- [PRD: Out of Scope](../PRD-vrg.md)
- [Help overlay (Issue #31)](help-overlay.md)
- [Outcome contract (Issue #9)](outcome-contract.md)
- [Record robustness (Issue #10)](record-robustness.md)
- [Safe presentation (Issue #6)](safe-presentation.md)
- [CLI flag forwarding (Issue #2)](cli-flag-forwarding.md)
