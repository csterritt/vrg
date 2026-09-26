# Documentation — README, help footer note, and synchronization tests (Issue #34)

Issue #34 (`Notes/issues/034-documentation-scale-and-memory-limits.md`,
tasks `Notes/tasks/034-documentation-scale-and-memory-limits.md`)
delivered the project's single user-facing documentation artifact —
`README.md` at the repository root, not a `docs/` page — plus the help
overlay's footer note, and the synchronization tests that keep both
honest. PRD cross-references in `Notes/PRD-vrg.md`: *Resources and
responsiveness*, *Out of Scope*, and *Further Notes*; the documented
behavior itself is *Invocation and child arguments*, *Outcome and
exit-status contract*, and *Colours, overlays, and key precedence*.

## The README

`README.md` documents:

- **ripgrep family** — ripgrep 15.x is the reference family; every
  search runs `rg --json --no-config <flags> -- <pattern> <root>` and
  VRG's supplied `--no-config` means ripgrep configuration files are
  never honoured (see
  [cli-flags-and-child-argv.md](cli-flags-and-child-argv.md)).
- **Invocation** — `vrg [flags] pattern [root]`; root defaults to `.`,
  accepts directories and regular files (including symlink targets),
  and rejects `-` (stdin). Options anywhere before `--`, forwarded in
  encounter order with supplied spellings; combined shorts expand left
  to right; `--` protects dash-leading operands; a cumulative
  `-u`/`--unrestricted` cap of two; everything else — including `=`
  assignment spellings such as `--ignore-case=false` — is a usage
  error.
- **Help-only path** — bare `vrg`, or `-h`/`--help` anywhere before
  `--`, prints the generated command-line help to stdout (exit 0)
  without running a search or starting the TUI — documented as
  distinct from the TUI's `h`/`?` help dialog. A flags-only invocation
  with no pattern (`vrg -i`) is a usage error, exit 2.
- **Generated help verbatim** — the README embeds the complete
  `vrg --help` output, so the flag allow-list with its exact
  no-argument spellings is the shared `optionDecls` declarations' own
  output rather than a copy (see
  [cli-foundation.md](cli-foundation.md)).
- **Key bindings** — the full `helpBindings` table (see
  [help-overlay.md](help-overlay.md)).
- **Exit statuses** — all four: 0 (successful search and browse, *and*
  command-line help), 1 (no usable results — "No results found"), 2
  (pre-TUI usage/root/start failures including the flags-only
  missing-pattern invocation, and fatal search outcomes), 130
  (cancellation — `q` while searching or result preparation is
  incomplete, `ctrl+c` in any state). The search-derived values agree
  with the Issue #9 `decideOutcome` function (see
  [error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)).
- **Scale and limits** — the three *independent* scale examples
  (approximately 10,000 matched files, 100,000 matched lines,
  individual files around 50 MB), explicitly not simultaneous capacity
  guarantees; the ~50 MB example's UTF-8/ordinary-line-length
  assumption with the base64 `bytes` expansion caveat that can push a
  single `match` record over the 64 MiB record limit, and the
  oversized-record diagnostic naming the path when recoverable (see
  [record-robustness.md](record-robustness.md)); session-long buffer
  retention with no eviction, no aggregate memory bound, no reliable
  OOM recovery, and no guaranteed terminal cleanup under forced
  termination.

## The help overlay footer note

`help.go` gained `limitNotes` — the three scale/record-limit/memory
statements as one structured source — and `helpFooter` is now filled
with it. `helpLines` still routes each footer entry through
`present.Diagnostic` (the Issue #6 utility), so the note is also the
help text's runtime-substitution path. The README carries the same
three statements verbatim as the *Scale, records, and memory* bullets,
and `TestHelpFooterMatchesREADME` asserts every installed footer entry
appears verbatim in both the composed help text and the README, so
neither sink can drift from the other (see
[help-overlay.md](help-overlay.md)).

## Synchronization tests

- `internal/app/readme_test.go` — `TestREADMEDocumentsEveryKeyBinding`
  iterates Issue #31's `helpBindings` and requires every key spelling
  and description in the README;
  `TestREADMEExitStatusDocumentation` requires the four table rows and
  drives representative completed-search inputs through the real
  `decideOutcome`, asserting every status it produces has a documented
  row, plus the trigger tokens (both exit-0 reasons, the `vrg -i`
  flags-only usage error, `q`-while-searching and `ctrl+c`
  cancellation); `TestREADMEDocumentsHelpOnlyPath` pins the help-only
  contract and its distinction from the `h`/`?` dialog;
  `TestREADMERipgrepReference`,
  `TestREADMEScaleExamples`, `TestREADMERecordLimit`, and
  `TestREADMEMemoryLimits` pin the family/`--no-config` statements and
  the AC1–AC3 scale, record-limit, and memory statements;
  `TestHelpFooterMatchesREADME` is the footer↔README equivalence
  check including the rendered overlay.
- `internal/cli/readme_test.go` (same package) —
  `TestREADMEDocumentsEveryDeclaredOption` iterates `optionDecls` and
  requires every declared spelling (search flags and the local help
  options) with its description and generated option line in the
  README; `TestREADMECarriesGeneratedHelp` requires the complete
  `HelpText()` block verbatim;
  `TestREADMEInvocationContract` pins the synopsis and the
  assignment-rejection statement.
- `internal/app/sinksafety_test.go` gained Issue #34's row,
  `help footer note`: `renderHelpFooterSink` substitutes each hostile
  fixture at every runtime-substitution point of the rendered footer —
  appended to every installed `limitNotes` entry plus a dedicated
  injection — scrolls the real overlay to the footer's rows, and the
  check asserts the `Diagnostic`-escaped `wantDiag` forms and the real
  note's `64 MiB` text inside the border under the shared no-style and
  styled passes (see [safe-presentation.md](safe-presentation.md)).

`help_test.go`'s `TestHelpScrollsWithUpDown` now reaches the footer at
the tail (passing the `ctrl+c` binding row on the way) instead of
ending on it, and the footer-slot tests save and restore `helpFooter`
now that it has real content.

## Manual verification

The issue's manual checks — every flag matching the allow-list and the
shared declarations (including local help options and assignment-form
rejection), the help-only behavior and its distinction from the TUI
dialog, every key matching the overlay, every exit status matching the
outcome table with both exit-0 reasons, and the ripgrep 15.x /
`--no-config` statements — are exercised in the Issue #34 walkthrough
at `Notes/walkthroughs/034-04/code-walkthrough`, which also shows the
footer note rendering in the overlay and the sink row against the
hostile fixtures.

## Files

- `README.md` — the user-facing documentation artifact.
- `internal/app/help.go` — `limitNotes` and the filled `helpFooter`.
- `internal/app/readme_test.go`, `internal/cli/readme_test.go`,
  `internal/app/sinksafety_test.go` — the synchronization tests and
  the Issue #34 sink row.

See also: [help-overlay.md](help-overlay.md) (the footer slot this
fills), [safe-presentation.md](safe-presentation.md) (the sink-safety
table the new row joins),
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
(the outcome function the exit table agrees with),
[cli-flags-and-child-argv.md](cli-flags-and-child-argv.md) (the
allow-list the README documents), and
[record-robustness.md](record-robustness.md) (the 64 MiB limit the
notes describe).
