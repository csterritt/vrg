# Tasks for #2: CLI flag allow-list, combined shorts, cumulative `-u`, `--`, ordered child argv

Parent issue: #2
Parent PRD: PRD-vrg.md
**Blocked by issues**: #1
**Acceptance criteria**: AC1 → RED 1 / GREEN 2; AC2 → RED 1 / GREEN 2; AC3 → RED 1 / GREEN 2; AC4 → RED 1 / GREEN 2; AC5 → RED 1 / GREEN 2; AC6 → RED 1 / GREEN 2; AC7 → RED 1 / GREEN 2; AC8 → RED 1 / GREEN 2; AC9 → RED 1 / GREEN 2; AC10 → RED 1 / GREEN 2; AC11 → RED 1 / GREEN 2

## Tasks

### 1. Specify ordered search flags, help regressions, and protected child argv

**Type**: RED
**Output**: Failing public-contract tests cover all eleven Issue #2 acceptance criteria: shared-declaration-driven flags/help, ordered exact-spelling forwarding, combined shorts, cumulative `-u`, lexical assignment rejection, help precedence, `--`, and exact child argv; every Issue #1 help/output sentinel remains green.
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issue #1 is complete. Extend the existing `internal/cli` behavioral suite without replacing Issue #1's shared declaration, preflight, generated-help, output-safety, or subprocess tests. Add table rows for every accepted short and long search flag and rejected options such as `-e` and argument-taking flags. Assert exact public child argv rather than private callback logs for repeated and mixed options (`-i -s -i`, `-isi`, and `--ignore-case -s -i`) and options interleaved with both operands (`vrg foo -i src -s`), proving encounter order and each supplied short/long spelling survive. Cover combined expansion left to right, including `-iwF`, and require mandatory child order `--json --no-config <ordered expanded user flags> -- <pattern> <root>` without normalization of contradictory flags.

Test cumulative unrestricted counting from ordered scan records across separate, combined, short, and long spellings: accepted `-u`, `-uu`, `-iu`, `-iuu`, and `-u --unrestricted`; rejected `-uuu`, `-u -uu`, `-iuuu`, `-u --unrestricted -u`, and `--unrestricted -uu`. Require the third occurrence to produce a sanitized exit-2 usage result and no child or TUI.

Add lexical rejection rows for boolean assignment forms, including `--ignore-case=false`, `-i=false`, `--unrestricted=false`, `--help=false`, and `-h=false`, and rows proving those same bytes remain positional after the first `--` when arity permits. Cover the literal `-` pattern, protected dash-leading patterns, a second `--` as the verbatim pattern in both default-root and explicit-root forms, and the empty pattern as an empty argv element.

Retain every Issue #1 help-only regression unchanged and add search-flag combinations: bare invocation, first-token `-h`/`--help`, `-i --help`, `-ih pattern`, help after a root, invalid root, excess operands, or unsupported option, and help-like operands after `--`. Require help before allow-list validation; exit 0; exactly one help copy on stdout; empty stderr; no child argv, root validation, success stub, or TUI; and `vrg -i` without help to remain a missing-pattern exit-2 error. Add a generated-help contract that iterates the same shared option declarations used by scanning and parser configuration and finds every allow-listed flag in short and long form, while preserving the Issue #1 syntax, positional, local-help, sanitization, and emission-path assertions. Keep this task test-only.

---

### 2. Extend the shared declarations and ordered preflight to build child argv

**Type**: GREEN
**Output**: Task 1 and all unchanged Issue #1 CLI/output/subprocess tests pass; the stub prints exact protected child argv built from declaration-backed ordered scan records, and generated help lists every search flag.
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Extend, rather than replace, Issue #1's shared option-declaration table and single ordered raw-token preflight. Add every supported no-argument search flag to the same declarations that configure `mow.cli`, validate raw tokens, and render generated command-line help; do not create an independently maintained allow-list. Scan argv left to right only up to the first `--`, recognize local literal or combined help before all search-flag validation, reject `=` assignment spellings lexically without treating help assignments as help, record each accepted search spelling in encounter order, and expand combined short tokens left to right. Preserve the exact short or long spelling supplied for uncombined flags.

Derive forwarding order and cumulative unrestricted counts solely from the ordered scan records, never from `VarOpt` callback order or values. Keep `mow.cli` responsible for declared syntax and parsing, and ensure any custom boolean value satisfies its boolean-option contract, but do not rely on library repetition limits or permissive boolean assignment parsing. Reject a third unrestricted occurrence across any token/alias mix. Preserve Issue #1's help-only result and selected native-output strategy without invoking root validation or search dispatch on help, and keep all tokens after the first `--` positional.

Build child arguments exactly as `--json`, `--no-config`, ordered expanded user flags, `--`, pattern, root; retain empty, literal `-`, dash-leading protected, and literal `--` patterns verbatim. Update the success stub to print this public argv and leave rg startup and TUI behavior to later issues. Run the focused CLI suite, all Issue #1 generated-help/output/subprocess regressions, `go test ./...`, `go vet ./...`, and `go build ./...`.

---

### 3. Create the complete flag-parsing finish marker

**Type**: FINISH MARKER
**Output**: Finish marker exists at `Notes/finish-markers/002-04/finish-marker.md`.
**Depends on**: 2

Write `Task 002-04 finished successfully at <time>` to `Notes/finish-markers/002-04/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/002-04/` directory if it does not already exist.

---
