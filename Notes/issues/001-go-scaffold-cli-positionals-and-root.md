## Issue 1: Go scaffold, CLI positionals, root default/validation, usage errors

**Type**: HITL
**Blocked by**: None — can start immediately

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

Bootstrap the greenfield Go project (module, `cmd/vrg` main, Bubble Tea/Bubbles/Lip Gloss dependencies, package layout for the six modules named in *Module Design*) and deliver the first end-to-end path through the CLI module: `vrg pattern [root]`.

- Parse positionals: first is the pattern, second is the root, root defaults to `.`. Missing pattern or more than two positionals is a usage error. A **present but empty** pattern argument is a present pattern, not a missing one: `vrg "" .` parses successfully and carries an empty pattern (forwarding it verbatim in the child argv is Issue 2).
- Validate the root before any TUI starts: existing directory or regular file (symlinks resolving to either accepted). Reject `-` (stdin), special files, and nonexistent paths. An actual regular file named `-` is addressed as `./-` and accepted.
- Usage errors print a sanitized single-line diagnostic to stderr and exit 2 with no TUI. Invalid bytes/control characters in the offending operand must be escaped (a minimal path escaper is acceptable here; Issue 6 generalises it).
- On success the binary may, for now, print the resolved pattern and root and exit 0 — enough to prove the slice end-to-end. Later issues replace this stub.

See PRD *Implementation Decisions → Invocation and child arguments* (first and sixth bullets) and *Module Design → CLI*.

**Human decision required**: confirm module path, package layout (e.g. `internal/cli`, `internal/searchindex`, …), Go version, and pinned Bubble Tea / Bubbles / Lip Gloss versions before merging.

### How to verify

- **Manual**:
  1. `go build ./...` succeeds; `go test ./...` passes.
  2. `vrg foo` in any directory prints the stub success line and exits 0.
  3. `vrg` → stderr usage diagnostic, exit 2. `vrg a b c` → exit 2.
  4. `vrg foo /nonexistent` → exit 2; `vrg foo -` → exit 2 with a stdin-rejection message; `vrg foo /dev/null` → exit 2.
  5. `vrg foo ./go.mod` (regular file) → exit 0; a symlink to a directory → exit 0.
  6. `touch ./- && vrg foo ./-` → exit 0 (an actual regular file named `-`, addressed as `./-`).
  7. `vrg "" .` → stub success with an empty pattern, exit 0 (a present empty pattern is not a missing pattern).
- **Automated**: CLI package tests cover default root, directory/regular-file/symlink roots, stdin and special-file rejection, nonexistent root, arity errors, the `./-` regular-file root, an empty-string pattern parsing as a present positional (not a usage error), and that every error message contains no raw control bytes. A `main`-level test (or subprocess test) asserts exit status 2 and stderr content for a usage error.

### Acceptance criteria

- [ ] Given `vrg pattern`, when the CLI parses, then root resolves to `.`.
- [ ] Given `vrg pattern dir` where `dir` is a directory or regular file (directly or via symlink), then validation succeeds.
- [ ] Given no pattern or more than two positionals, then a sanitized usage diagnostic is written to stderr and the process exits 2 without starting a TUI.
- [ ] Given a root of `-`, a special file, or a nonexistent path, then the process exits 2 with a sanitized diagnostic naming the reason.
- [ ] Given a root operand containing control bytes, then the diagnostic escapes them rather than emitting them raw.
- [ ] Given a root of `./-` naming an actual regular file called `-`, then validation succeeds.
- [ ] Given a present but empty pattern argument, then it is treated as a present pattern and parsing succeeds without a usage error.

### Definition of done

- [ ] The human decision above is resolved: the reviewer's approval of module path, package layout, Go version, and pinned Bubble Tea / Bubbles / Lip Gloss versions is recorded before merge.

### User stories addressed

- User story 1: search a directory or regular file
- User story 2: root defaults to `.`
- User story 3: usage diagnostics on stderr with exit 2
- User story 8 (part): stdin roots rejected

---
