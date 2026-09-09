## Issue 35: Final integration verification — clean repository-wide build, vet, test, and smoke run

**Type**: AFK
**Blocked by**: Issue 30, Issue 33, Issue 34

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

The single closing verification pass over the fully composed implementation. Issues #30, #33, and #34 are the dependency graph's terminal issues, so blocking on them transitively requires every feature issue (#1–#34) to have landed first. No new product behavior is built here; this issue owns the proof that the assembled repository — not an earlier feature slice — builds, vets, tests, and launches.

- From a clean checkout of the completed repository, run `go build ./...`, `go vet ./...`, and `go test ./...`. All three must pass over the fully composed implementation. A regression in any package or test fails this issue's closure; previously green focused tests do not substitute for a clean full-suite pass.
- Run the critical PTY/subprocess tests (Issues #4, #9, and #11) explicitly — not skipped, not short-circuited, and not satisfied only by cached results — without relying on sleeps, per the PRD's Testing Decisions.
- Build the final `vrg` binary and smoke-run it end to end through the five representative outcomes, using the Issue #4 fake-rg harness for determinism: a successful browse (exit 0 after `q`), a no-results search (exit 1 after `q`), a fatal fake-rg outcome with no usable results (exit 2 after overlay dismissal with either `q` or `Esc`), cancellation while searching (exit 130 with the child terminated and reaped and the terminal restored), and a help-only invocation (bare `vrg`, `-h`, and `--help` → exactly one help copy on stdout, empty stderr, exit 0) run with ripgrep unavailable on `PATH` and a sentinel fake rg available but never invoked, proving the final binary's no-child/no-TUI startup branch. Each smoke run asserts the exit status, terminal restoration, and stderr replay where applicable.
- Record every command and its results in the final walkthrough artifact; that record is the closing review evidence for the whole task set.

A regression discovered by this pass is fixed here only insofar as it restores the composed contract of the owning issue, with the full suite rerun green afterwards — never by patching, weakening, or deleting a single focused test.

See PRD *Testing Decisions* (subprocess boundary, responsiveness boundaries) and *Module Design*.

### How to verify

- **Manual**: from a clean checkout, run the three repository-wide gates and record the commands, outputs, and exit statuses; then build the final binary and run the smoke scenarios with the fake-rg harness — the four search outcomes plus the help-only invocation with ripgrep unavailable and a sentinel fake rg present but never started — recording each exit status and confirming the terminal is usable afterwards.
- **Automated**: the composed suite from Issues #1–#34 is the contract — `go test ./...` from the clean checkout is the pass, with the PTY/subprocess packages explicitly re-run (for example with `-count=1`) so no cached result stands in for an executed run. This issue adds no new product tests; any regression it uncovers is repaired against the owning issue's existing contract.

### Acceptance criteria

- [ ] Given a clean checkout of the completed repository, then `go build ./...`, `go vet ./...`, and `go test ./...` all pass over the fully composed implementation.
- [ ] Given the final verification run, then the critical PTY/subprocess tests execute explicitly without sleeps — not skipped, short-circuited, or satisfied only by cached results.
- [ ] Given the final `vrg` binary, then smoke runs of a successful browse (0), a no-results search (1), a fatal fake-rg outcome (2), cancellation (130), and a help-only invocation (bare `vrg`, `-h`, `--help` → one help copy on stdout, empty stderr, exit 0, no child, no TUI) succeed with terminal restoration and, where applicable, stderr replay.
- [ ] Given any regression found by the pass, then it is fixed against the owning issue's contract and the full suite is rerun green; closure fails otherwise.
- [ ] Given the completed pass, then every command and its results are recorded in the final walkthrough artifact.

### User stories addressed

- User stories 1–85: the fully composed program specified by the PRD is proven to build, vet, test, and launch end to end.

---
