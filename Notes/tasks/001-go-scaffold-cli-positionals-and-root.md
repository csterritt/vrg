# Tasks for #1: Go scaffold with mow.cli, default help, positionals, and root validation

Parent issue: #1
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Acceptance criteria**: AC1 → RED 3 / GREEN 4 (CONFIG 2 establishes the pin); AC2 → RED 3 / GREEN 4; AC3 → RED 3 / GREEN 4; AC4 → RED 3 / GREEN 4; AC5 → RED 3 / GREEN 4; AC6 → RED 3 / GREEN 4; AC7 → RED 3 / GREEN 4; AC8 → RED 3 / GREEN 4; AC9 → RED 3 / GREEN 4; AC10 → RED 3 / GREEN 4; AC11 → RED 3 / GREEN 4; AC12 → RED 3 / GREEN 4; AC13 → RED 3 / GREEN 4; AC14 → RED 3 / GREEN 4; AC15 → RED 3 / GREEN 4
**Manual verification**: Task 6 owns the issue's manual checks. Task 1 records the architecture decision before implementation; Task 7 verifies the result against it.

## Tasks

### 1. Approve the scaffold and CLI output architecture

**Type**: REVIEW
**Output**: A human-approved decision records the module path, six-package layout, Go version, exact pinned `mow.cli` / Bubble Tea / Bubbles / Lip Gloss versions, and one complete native-output prevention or containment strategy before architecture-dependent work starts.
**Depends on**: none

Review Issue #1 and the Invocation and child arguments, Module Design, and Testing Decisions sections of `Notes/PRD-vrg.md`. Record the module path, package layout for CLI, SearchIndex, FileBuffer, Viewport, Theme, and App, Go version, and exact dependency versions, including `github.com/jawher/mow.cli v1.2.0`. Select either file-descriptor containment for every library emission point, with process-wide serialization, concurrent pipe drainage, and reliable restoration on every controlled path, or metadata rendering from the same declarations that configure `mow.cli`, paired with preflight validation that makes every native help and parse-failure emission path unreachable. Confirm how tests will prove the selected strategy handles first-token help, parser failure, explicit help rendering, hostile substitutions, and concurrent/restoration obligations where applicable. Do not begin Task 2 until this decision is recorded.

---

### 2. Initialize the approved Go module and package layout

**Type**: CONFIG
**Output**: `go.mod`, `cmd/vrg/main.go`, and buildable package boundaries for all six PRD modules exist; all approved dependencies are pinned, including `github.com/jawher/mow.cli v1.2.0`; `go mod verify`, `go build ./...`, and `go vet ./...` pass.
**Depends on**: 1

Initialize the greenfield project exactly as approved in Task 1. Create the minimal executable entry point and package boundaries under `internal/` for the six PRD responsibilities. Add the approved, mutually compatible dependencies through Go tooling with exact versions, including `github.com/jawher/mow.cli v1.2.0`, Bubble Tea, Bubbles, and Lip Gloss. Keep the packages minimal and buildable; the dependency may be wired through a compile-safe CLI adapter seam, but behavioral parsing, output, child startup, and rendering belong to Tasks 3–4 and later issues. Verify the module graph and run the build and vet gates.

---

### 3. Specify the complete CLI foundation and executable boundary

**Type**: RED
**Output**: Failing CLI, output, adapter, and subprocess tests cover all fifteen Issue #1 acceptance criteria, including generated help and precedence, explicit result kinds, controlled native emissions, positional/root behavior, sanitization, and entry-point-owned statuses and side effects.
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add externally observable tests in `internal/cli` and at the `cmd/vrg` process boundary before production implementation. Require the adapter to use `mow.cli` behind the CLI interface with `flag.ContinueOnError`, and require distinct help-only, parsed-search, and classified usage-error results so `cmd/vrg` alone selects status and stream and cannot fall through from help into the success stub. Assert help content includes the syntax, required `PATTERN`, optional `ROOT` with default `.`, and local `-h`/`--help`, rendered from the shared option/argument declarations that also configure the parser and can be extended by Issue #2.

Cover bare invocation; first-token `-h` and `--help`; later help after a pattern, invalid root, excess operands, or unsupported option; flag-preceded help without a pattern (`-i --help`); combined help (`-ih`); and help-like tokens after `--`. For every help-only row, use failing sentinels for root validation, child startup, TUI initialization, and success-stub dispatch. At subprocess scope require exit 0, exactly one help copy on stdout, empty stderr, no alternate-screen or stub output, operation with rg absent from `PATH`, and no invocation of a sentinel fake rg. Include the library-behavior seam showing why first-token interception and a successfully parsed local-help value cannot be treated as an ordinary successful search result.

Test the approved Task 1 output strategy across every possible `mow.cli` emission point: first-token help, parser-failure `Error:` plus help, and module-requested help. Require no native library bytes or duplicate diagnostics on process stderr, exactly one sanitized help copy on stdout, and only the module's specific sanitized single-line diagnostic on each usage error. Cover hostile operand and executable-name substitutions, invalid bytes, and terminal controls while preserving legitimate help line breaks. For containment, add deterministic tests for serialization, capture drainage beyond pipe capacity, and restoration after success and failure; for prevention, prove preflight classifies every help and invalid-input path before any emitting library call and that help renders from the parser's shared metadata.

Add table tests for root defaulting to `.`, existing directory/regular-file/symlink roots, invalid/nonexistent/special roots, stdin root `-`, a real `./-` file, present empty and literal `-` patterns, missing pattern and excess operands outside help-only paths, unsupported options, and `--` making `-h`, `--help`, and another `--` positional. Require specific diagnostic kinds rather than `incorrect usage`, exit 2 with no child or TUI for errors, and a safely escaped success stub for valid parses. Keep this task test-only and retain named test groups for generated CLI-help stdout and CLI output safety so Issue #6's tasks can rerun them unchanged.

---

### 4. Implement the mow.cli adapter, preflight, help, and validated parse

**Type**: GREEN
**Output**: Task 3's focused and subprocess suites pass; the runnable binary uses `mow.cli v1.2.0`, emits controlled generated help, returns explicit result kinds, validates positionals/root, and leaves all statuses and process side effects to `cmd/vrg`.
**Depends on**: 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement the CLI foundation required by Task 3 and no later feature behavior. Keep `mow.cli` types behind the CLI adapter, configure its application with `flag.ContinueOnError`, and make `cmd/vrg` a thin boundary that dispatches explicit help-only, parsed-search, and usage-error results to the correct stream/status. Establish one extensible option/argument declaration table as the source for parser configuration, raw-token recognition, and generated help. Establish one ordered raw-token preflight that scans left to right to the first `--`, recognizes literal and combined local help before allow-list, arity, root, or search work, and is designed for Issue #2 to add ordered search-flag records and cumulative counts without replacement.

Apply the Task 1 output architecture completely rather than relying on `ContinueOnError` to suppress writes. Ensure no library call can leak first-token help, parser diagnostics, or explicit help output; sanitize every runtime substitution. Handle no arguments before library parsing, make preflight help authoritative, and check any successfully parsed local-help value before validation or action side effects. Classify missing pattern, excess operands, unsupported option, and invalid root in the module; never expose the library's generic error. Parse first positional as pattern and second as root defaulting to `.`, retain empty and literal `-` patterns, honor the first `--`, validate accepted root types and symlinks, reject stdin/special/nonexistent roots, and print only the interim safely escaped pattern/root stub for a parsed search. Run focused tests, subprocess tests, `go test ./...`, `go vet ./...`, `go build ./...`, and `go mod verify`.

---

### 5. Document the CLI foundation in the wiki

**Type**: DOCUMENT
**Output**: Wiki pages record the approved architecture, `mow.cli` adapter and output strategy, shared declarations/preflight, help and result contracts, positionals/root validation, tests, and scaffold layout.
**Depends on**: 4

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #1 implementation and tests into the appropriate pages under `Notes/wiki`. Document the approved module/dependency choices including `mow.cli v1.2.0`; the adapter boundary, `ContinueOnError`, entry-point exit ownership, selected native-output strategy, shared declarations, ordered preflight, and explicit result kinds; bare and local help precedence and exact stdout/stderr/no-side-effect behavior; generated help content and distinction from TUI help; positional, `--`, root, sanitization, and stub contracts; and the named CLI-help/output and subprocess suites that later tasks consume. Cross-reference Issue #1 and the relevant PRD sections, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 6. Create the complete CLI-foundation walkthrough

**Type**: CODE WALKTHROUGH
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/001-06/code-walkthrough` and records every Issue #1 manual verification class plus the focused output and executable-boundary suites.
**Depends on**: 5

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/001-06/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate module verification, build, vet, focused CLI/output tests, and subprocess tests. Build the binary and capture stdout, stderr, and status separately for bare, first-token, later-token, combined, invalid-root-plus-help, missing-pattern, excess-operand, unsupported-option, invalid root classes, default/explicit/symlink/regular-file roots, `./-`, empty and literal `-` patterns, and help-like operands after `--`. Show that all help cases emit one help copy on stdout with empty stderr, no child/stub/TUI, and work with rg unavailable; show specific sanitized exit-2 diagnostics and hostile-control escaping for errors. Record that the selected output strategy and explicit result dispatch match Task 1's decision. Reference Issue #1 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

### 7. Review implementation against the recorded decision

**Type**: REVIEW
**Output**: Human approval confirms the implementation, tests, wiki, and walkthrough satisfy Issue #1 and the architecture recorded in Task 1 before Issue #2 starts.
**Depends on**: 6

Review the completed scaffold and CLI slice against every Issue #1 acceptance criterion and the Task 1 record. Confirm the module path, six-package layout, Go version, all exact dependency pins including `mow.cli v1.2.0`, actual use of the adapter, `ContinueOnError`, shared declaration/preflight extension seam, explicit result kinds, and the complete native-output strategy with any serialization, drainage, restoration, or emission-prevention proof it requires. Confirm the named generated-help/CLI-output tests and subprocess sentinels exist for downstream Tasks #6, #34, and #35. Record approval or require corrections before the pipeline proceeds to Issue #2.

---
