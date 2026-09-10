# Decision record: Issue #1 scaffold and CLI output architecture

**Date**: 2026-09-10
**Status**: Approved by human reviewer (Task 1 gate, Issue #1)
**References**: `Notes/issues/001-go-scaffold-cli-positionals-and-root.md`, `Notes/tasks/001-go-scaffold-cli-positionals-and-root.md`, `Notes/PRD-vrg.md` (revision 6) — *Implementation Decisions → Invocation and child arguments*, *Module Design → CLI*, *Testing Decisions → CLI*.

## Approved decisions

### Module path

`vrg` — a short local module path for a greenfield binary that is not fetched as a library.

### Package layout

```
cmd/vrg                 thin process boundary: argv/stdout/stderr/exit-status ownership only
internal/cli            mow.cli adapter: preflight, parsing, generated help, root validation, results
internal/searchindex    SearchIndex module boundary (later issues)
internal/filebuffer     FileBuffer module boundary (later issues)
internal/viewport       Viewport module boundary (later issues)
internal/theme          Theme module boundary (later issues)
internal/app            App module boundary (later issues)
```

All six PRD Module Design responsibilities get a package boundary now; only `cli` and `cmd/vrg` receive behavior in Issue #1.

### Go version

`go 1.27.1` in `go.mod` (installed toolchain go1.27.1, linux/arm64).

### Dependency pins (exact versions, added via `go get`)

| Module | Version |
|---|---|
| `github.com/jawher/mow.cli` | `v1.2.0` (PRD-mandated) |
| `charm.land/bubbletea/v2` | `v2.0.9` |
| `charm.land/bubbles/v2` | `v2.2.1` |
| `charm.land/lipgloss/v2` | `v2.0.6` |

The v2 Charm stack is the current stable generation and its three modules are mutually compatible; choosing it now avoids a v1→v2 migration during the later TUI issues. The Charm v2 modules publish under `charm.land/` module paths (the `github.com/charmbracelet/*` v2 paths are aliases whose go.mod files declare the `charm.land` identity).

## Native-output strategy: emission prevention (option b)

Chosen over file-descriptor containment (option a). The issue records a preference for an approach that does not introduce unsynchronized global stderr mutation into a reusable CLI module; prevention satisfies the stdout-and-sanitization contract without swapping process file descriptors, so the serialization, concurrent pipe-drainage, and restoration obligations of containment do not apply.

The library's three emission points and how each is made unreachable or unnecessary:

1. **First-token help** (`PrintLongHelp` inside `Cmd.parse`): unreachable. An ordered raw-token preflight runs before `app.Run`, scans argv left to right stopping at the first `--`, and resolves `-h`, `--help`, and combined short tokens containing `h` to the module's own help-only result. No argv that could reach the library's interception is ever passed to `Run` — including every token the library would treat as first-token help, because the module intercepts help for all pre-`--` positions.
2. **Parser-failure output** (`Error: …` + `PrintHelp` before returning the error): unreachable. The same preflight validates every token the library parser could reject — unsupported option spellings, invalid boolean assignments to `-h`/`--help`, missing required `PATTERN`, and excess positionals — and classifies them into the module's own usage-error results before `Run` is invoked. `Run` sees only input already proven acceptable against the shared declarations, so the FSM failure path cannot fire.
3. **Explicit help rendering**: the module renders command-line help itself from the same option/argument declaration table that configures `mow.cli` (single shared source, no drift), and never calls the library's `PrintHelp`/`PrintLongHelp`.

Supporting choices:

- `cli.App` is constructed with the fixed name `"vrg"`, so `os.Args[0]` can never reach output as a substitution.
- `ErrorHandling = flag.ContinueOnError`: the library never calls `os.Exit`; `cmd/vrg` owns every exit status. `ContinueOnError` is relied on for exit behavior only — emission prevention does not depend on it suppressing writes (it does not).
- `mow.cli` types stay behind the `internal/cli` adapter; the module returns an explicit result kind — help-only, parsed-search, or classified usage-error — and `cmd/vrg` alone selects output stream and status.
- A successfully parsed local-help value (e.g. `--help=true`) re-checks the declared help option inside the adapter before root validation or any search work, and yields the same help-only result.
- Nothing external (argv values, executable name) reaches any output unsanitized; a minimal escaper inside `internal/cli` covers operand/diagnostic/stub substitutions until Issue #6 generalizes safe presentation.

## How tests prove the strategy

- **Subprocess tests** execute the built `cmd/vrg` binary and assert, for every emission point — first-token help, later-token/combined help, parser-failure classes, hostile operand and executable-name substitutions — that process stderr is empty, stdout carries exactly one sanitized help copy on help-only paths, and usage errors carry only the module's classified single-line diagnostic.
- **Unit tests** use failing sentinels (root validation, success-stub dispatch) to prove the preflight classifies every help and invalid-input path before any library call, and that the help-only result is authoritative ahead of arity, option allow-list, and root checks.
- **Help rendering** is asserted to come from the shared declaration metadata (syntax line, required `PATTERN`, optional `ROOT` with `.` default, `-h`/`--help`), extensible by Issue #2.
- Containment-specific obligations (serialization, drainage, restoration) are not applicable under prevention; unreachability is proven instead by the empty-stderr assertions and the preflight-before-`Run` sentinels above.

## Extension seam reserved for Issue #2

The option-declaration table and the single ordered raw-token scan are designed to grow: Issue #2 appends the allow-listed no-argument search flags to the same declarations (configuring `mow.cli`, driving scan validation, and rendering generated flag help) and records accepted spellings in encounter order for child argv, without replacing the scan or the table.

---

## Task 7 verification record — 2026-09-10

Verified the completed implementation and tests against this record and
every Issue #1 acceptance criterion. Findings:

- **Scaffold**: module `vrg`, `go 1.27.1`; `cmd/vrg` thin boundary plus
  `internal/{cli,searchindex,filebuffer,viewport,theme,app}`. `go.mod`
  pins `github.com/jawher/mow.cli v1.2.0` (direct require) and the Charm
  v2 stack (`charm.land/bubbletea/v2 v2.0.9`, `charm.land/bubbles/v2
  v2.2.1`, `charm.land/lipgloss/v2 v2.0.6`, `// indirect` pending first
  imports in later issues). `go mod verify`, `go build ./...`,
  `go vet ./...`, `go test ./...` all pass.
- **Adapter & exit ownership**: `internal/cli` is the sole mow.cli contact
  (`newApp`, fixed name `vrg`, `Spec`, `BoolOptPtr`, `StringArgPtr`,
  `Run`). `flag.ContinueOnError` is set and pinned by
  `TestAppUsesContinueOnError`; `cmd/vrg` alone maps explicit result kinds
  (`KindHelp`/`KindSearch`/`KindUsageError`) to streams and statuses.
- **Output strategy (prevention)**: the preflight resolves every help
  request and validates every token/arity the library could reject before
  `Run`, making all three emission points unreachable: first-token
  `PrintLongHelp` (no `-h`/`--help` token ever reaches `Run`), FSM-failure
  `Error:`+`PrintHelp` (only provably parseable input is passed), and
  explicit help printing (help renders from `optionDecls`/`argDecls`; the
  library's help printers are never called; `Action` is always non-nil).
  Containment obligations (serialization/drainage/restoration) are not
  applicable. The load-bearing invariant — preflight completeness — was
  checked against the mow.cli matcher source for every accepted token
  class (lone `-`, `--`, `-h=`/`--help=` valid bool assignments,
  non-dash operands, post-`--` operands); a `Run` error is also handled
  defensively, though unreachable.
- **Shared declarations & preflight**: `optionDecls`/`argDecls` drive the
  spec, option declarations, raw-token help/supported-spelling
  recognition, and generated help; `scanArgs` is the single ordered scan
  designed for Issue #2's ordered search-flag records.
- **Help & precedence**: bare/first-token/later/combined help, help over
  missing-pattern/excess/unsupported/invalid-root, assignment-spelling
  distinctions (`--help=false`/`-h=false` not help; `--help=true` help via
  the parsed-value check), help-like operands after `--` — all covered by
  `internal/cli` tables and `cmd/vrg` subprocess rows (empty stderr, one
  help copy, no stub/TUI, rg-absent and fake-rg sentinels, hostile argv0).
- **Positionals/root/sanitization**: default `.`, dir/file/symlink roots,
  `-` stdin rejection, special/nonexistent rejection, `./-`, empty and
  `-` patterns, `Escape` coverage of diagnostics and the stub.
- **Named downstream suites**: `TestGeneratedHelpStdout`,
  `TestCLIOutputSafety`, `TestExecutableBoundary` in
  `cmd/vrg/main_test.go` exist for Issue #6/#34/#35 reuse.
- **Docs**: wiki pages (`cli-foundation`, `source-code`, `unit-tests`,
  `project-overview`, index, log) and the showboat walkthrough at
  `Notes/walkthroughs/001-06/code-walkthrough/walkthrough.md` (verified
  with `showboat verify`, exit 0) are in place.

Corrections applied during implementation review: `isHelpToken` initially
accepted `-h=false` as help (saw `h` before the letters-only check) — now
verifies the whole token is ASCII letters first; help-token recognition
was made table-driven via the `help` flag on `optionDecl`. One test
expectation was fixed (`foo -- --help` classifies the operand as an
invalid root, not a search).

**Outcome**: implementation satisfies Issue #1 and this record.

**Approved by human reviewer 2026-09-10** after one correction (usage
text appended to stderr diagnostics on usage errors). Issue #1 is complete;
the pipeline may proceed to Issue #2.

### Correction 2026-09-10 (review feedback)

Usage errors now print the sanitized diagnostic line **followed by the
generated usage block** on stderr (still exit 2, stdout stays empty), so
users see usage after a mistake exactly as mow.cli's own `Error:` + help
path would have shown it — but rendered from our declarations and
sanitized. `cli.HelpText()` exposes the shared-renderer output to
`cmd/vrg` for this. This is the "any additional usage text on error goes
to stderr" allowance in Issue #1, and does not change the prevention
strategy: it is module-rendered text on a stream the entry point owns.
