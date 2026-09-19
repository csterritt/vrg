# Documentation — README, help footer, and the scale/memory limits (Issue #34)

Delivered by
[Issue #34](../issues/034-documentation-scale-and-memory-limits.md)
([task](../tasks/034-documentation-scale-and-memory-limits.md)): the
repository `README.md` is the **single user-facing documentation
artifact** — not a `docs/` page — and the TUI help overlay's footer
note carries the same scale, record-limit, and memory statements.
Relevant PRD sections: *Resources and responsiveness*, *Out of Scope*,
and *Further Notes* in [`Notes/PRD-vrg.md`](../PRD-vrg.md); user story
85. Builds on [help-overlay.md](help-overlay.md) (the binding table
and footer slot), [cli-flags-child-argv.md](cli-flags-child-argv.md)
(the shared option declarations),
[error-overlay-and-outcomes.md](error-overlay-and-outcomes.md)
(`DecideOutcome`), [record-robustness.md](record-robustness.md) (the
64 MiB limit and oversized diagnostics), and
[safe-presentation.md](safe-presentation.md) (the sink-safety table
this issue extends).

## The shared structured source (`internal/docs`)

`internal/docs/docs.go` is the one structured source both sinks render
from: `Statements` — the scale-independence statement, the ~50 MB
UTF-8/ordinary-line-length assumption with the base64 `bytes`
expansion caveat, the 64 MiB record-limit statement with the
oversized-record diagnostic naming the path when recoverable, and the
session-retention/no-OOM/no-cleanup memory statement — plus
`ScaleItems` (about 10,000 matched files; about 100,000 matched lines;
individual files around 50 MB). `docs.Footer()` renders them as the
help overlay's footer paragraph; `docs.Limits()` renders the README's
scale-and-memory-limits section body. Because both sinks render the
same source — and the tests assert the committed README carries
`Limits()` verbatim while the composed help body carries every
statement — neither sink can drift from the other, and deleting any
statement fails the suite.

## What the README documents

- **Invocation and help-only exit 0.** `vrg [flags] pattern [root]`;
  bare `vrg` and `-h`/`--help` print command-line help on stdout with
  exit 0 — no search, no TUI — explicitly distinct from the TUI's
  `h`/`?` help dialog. A flags-only invocation (`vrg -i`) is the
  missing-pattern usage error, exit 2.
- **Flags.** The complete option table — the local help options and
  every allow-listed no-argument search flag — in exact spellings,
  asserted row-for-row against `cli.optionDecls` so it cannot drift
  from parsing; assignment forms (`--ignore-case=false`) documented as
  rejected.
- **Key bindings.** The binding table asserted row-for-row against
  `app.helpBindings`, the same table the help dialog renders.
- **Exit statuses.** All four: 0 (successful search and browse *and*
  command-line help), 1 (no usable results), 2 (pre-TUI usage/root/
  start failures including `vrg -i`, plus fatal search outcomes), and
  130 (`q` while searching or result preparation is incomplete,
  `ctrl+c` in any state). The documented search-derived values are
  asserted to agree with `DecideOutcome`.
- **ripgrep 15.x** as the reference family, and `--no-config` supplied
  by VRG so configuration files are never honoured.
- **Scale and memory.** The three independent scale examples (not
  simultaneous capacity guarantees), the 64 MiB record limit with the
  base64 caveat, the oversized diagnostic, and the session-retention /
  no-eviction / no-aggregate-bound / no-OOM-recovery /
  no-cleanup-under-forced-termination limits.

## The footer sink-safety row

Issue #34 owns the generated-documentation row of the Issue #6
sink-safety table: `sinksafety_test.go` gains the "help footer note"
row, driven by `footerFixtureView`, which substitutes each hostile
fixture at the rendered help footer's runtime-substitution point (the
`helpFooter` slot routed through `EscapeDiagnostic`) and asserts the
escaped bytes end the composed body and reach the frame. Ten rows now
— static text needs no sanitization; only generated text paths that
accept runtime strings join the table.

## Tests

`internal/docs/docs_test.go` asserts the committed README carries
`Limits()` verbatim and `Footer()` carries every statement and scale
item. `internal/app/readme_test.go` asserts every `helpBindings` row
appears as a README table row, the composed help body and the README
share every docs statement and scale item, and the exit-status rows
cover all four statuses with their triggers and agree with
`DecideOutcome` over the search-derived conditions. `internal/cli/
readme_test.go` asserts every `optionDecls` spelling and description
appears as a README flag-table row, the help-only path tokens, and the
ripgrep 15.x / `--no-config` statements. See
[unit-tests.md](unit-tests.md).
