# Tasks for #1: Go scaffold, CLI positionals, root default/validation, usage errors

Parent issue: #1
Parent PRD: PRD-vrg.md
**Blocked by issues**: none
**Acceptance criteria**: AC1–AC7 → Tasks 2–3
**Manual verification**: Task 5 owns the issue's manual checks; Task 6 records the required human decision.

## Tasks

### 1. Initialize the Go module and package layout

**Type**: CONFIG  
**Output**: `go.mod`, `cmd/vrg/main.go`, and working package boundaries for the six PRD modules exist with pinned Bubble Tea, Bubbles, and Lip Gloss versions; `go build ./...` and `go vet ./...` pass.  
**Depends on**: none

Initialize the greenfield Go project required by Issue #1 and the Module Design section of `Notes/PRD-vrg.md`. Create the module and a minimal `cmd/vrg` executable entrypoint, establish working package boundaries for the CLI, SearchIndex, FileBuffer, Viewport, Theme, and App responsibilities under `internal/` (for example `internal/cli`, `internal/searchindex`, `internal/filebuffer`, `internal/viewport`, `internal/theme`, and `internal/app`), and add vetted, mutually compatible Bubble Tea, Bubbles, and Lip Gloss dependencies through Go tooling with exact pinned versions. Keep every package minimal and buildable — this task configures the scaffold only, with no CLI parsing, spawning, or rendering behavior owned by later tasks. Finish by running `go build ./...` and `go vet ./...`.

---

### 2. Specify positionals, root validation, and usage failures

**Type**: RED  
**Output**: Failing table-driven tests cover the default root, directory/regular-file/symlink roots, stdin and special-file rejection, nonexistent roots, arity errors, the `./-` regular-file root, the present-empty-pattern rule, and sanitized diagnostics with no raw control bytes; a process-boundary test asserts exit status 2 and stderr content for a usage error.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Add table-driven tests in `internal/cli` plus a process-boundary test around `cmd/vrg` for the Issue #1 contracts and the Invocation and child arguments section of `Notes/PRD-vrg.md`. Cover the pattern-then-root positional order with the root defaulting to `.`, acceptance of directory, regular-file, and symlink-resolving roots, rejection of `-` as stdin and of special files with sanitized reason-naming diagnostics, missing patterns and more than two positionals as usage errors, an actual regular file named `-` addressed as `./-`, and a present but empty pattern argument parsing as a present positional rather than a usage error. Require every diagnostic to be a single sanitized stderr line escaping control bytes and invalid bytes in the offending operand, and require usage failures to exit 2 without starting a TUI. Keep root validation injectable so tests need no privileged setup, and keep this task test-only.

---

### 3. Implement the CLI parse, validation, and stub success

**Type**: GREEN  
**Output**: Positional, root-validation, usage-failure, and process-boundary tests pass; a successful parse prints the resolved pattern and root and exits 0.  
**Depends on**: 2

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Implement only enough CLI behavior in `internal/cli` to satisfy Task 2, keeping `cmd/vrg` a thin process boundary that maps parse and validation outcomes to exit statuses and streams. Parse positionals in order with the root defaulting to `.`, validate the root as an existing directory or regular file (following symlinks) before any TUI starts, reject `-`, special files, and nonexistent paths with sanitized single-line diagnostics and exit 2, and treat a present empty pattern as a present positional. Add the minimal operand escaper this issue permits (Issue #6 generalizes it), and on success print the resolved pattern and root as the interim stub and exit 0. Do not add flag parsing, child argv construction, or any TUI behavior owned by Issues #2–#5.

---

### 4. Document the CLI contract in the wiki

**Type**: DOCUMENT  
**Output**: Wiki pages record the invocation syntax, root validation rules, usage diagnostics, exit status 2, the stub success output, and the scaffold layout.  
**Depends on**: 3

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #1 scaffold, `internal/cli` implementation, and tests into the appropriate pages under `Notes/wiki`, creating any catalog page the schema defines that does not yet exist (such as `index.md`, `log.md`, `project-overview.md`, `source-code.md`, and `unit-tests.md`). Document the `vrg pattern [root]` positional contract, the root default and validation rules with their rejection classes, sanitized single-line usage diagnostics and exit status 2 without a TUI, the present-empty-pattern rule, the interim stub success output, and the module and package layout with its pinned dependencies. Cross-reference Issue #1 and the Invocation and child arguments and Module Design sections of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 5. Create the CLI walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/001-05/code-walkthrough`.  
**Depends on**: 4

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/001-05/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate `go build ./...`, `go vet ./...`, and the focused `internal/cli` and `cmd/vrg` tests, then exercise the built binary for the stub success with default and explicit roots, directory, regular-file, and symlink roots, the `./-` regular-file case, and the present empty pattern, plus usage failures for a missing pattern, excess positionals, `-`, a special file, and a nonexistent root with exit 2 and sanitized stderr. Reference Issue #1 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---

### 6. Review the scaffold decisions

**Type**: REVIEW  
**Output**: Human approval of the module path, package layout, Go version, and pinned Bubble Tea / Bubbles / Lip Gloss versions is recorded before merge.  
**Depends on**: 5

Review the scaffold, implementation, tests, wiki updates, and `Notes/walkthroughs/001-05/code-walkthrough` against Issue #1 and its required human decision. Confirm the module path, the package layout for the six PRD modules, the Go version, and the exact pinned Bubble Tea, Bubbles, and Lip Gloss versions, checking that the dependencies are mutually compatible and were added only through Go tooling. These choices bind every later issue's task file; record the approval, and any renames, before the pipeline proceeds to Issue #2.

---
