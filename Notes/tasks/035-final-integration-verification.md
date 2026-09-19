# Tasks for #35: Final integration verification — clean repository-wide build, vet, test, and smoke run

Parent issue: #35
Parent PRD: PRD-vrg.md
**Blocked by issues**: #30, #33, #34
**Acceptance criteria**: AC1–AC4 → Task 1

## Tasks

### 1. Run the clean repository-wide verification pass

**Type**: GREEN  
**Output**: From a clean checkout, `go build ./...`, `go vet ./...`, and `go test ./...` pass over the fully composed implementation with the PTY/subprocess tests explicitly executed, and the final binary's five smoke scenarios (browse 0, no results 1, fatal 2, cancellation 130, help-only 0) succeed; any regression is repaired against the owning issue's contract and the full suite reruns green.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #30, #33, and #34 are complete — they are the dependency graph's terminal issues and together transitively require every feature issue. The composed RED contracts of Issues #1–#34 are this task's contract. Start from a clean checkout of the completed repository (a fresh clone or fully cleaned worktree) so no stale artifact can mask a failure. Run `go build ./...`, `go vet ./...`, and `go test ./...`, then explicitly re-run the PTY/subprocess test packages from Issues #4, #9, and #11 with test caching disabled (for example `-count=1`) so every critical boundary test executes rather than being satisfied by a cached result, without sleeps. Build the final `vrg` binary and smoke-run it through the five representative outcomes with the Issue #4 fake-rg harness: a successful browse exiting 0 after `q`, a no-results search exiting 1 after `q`, a fatal fake-rg outcome with no usable results exiting 2 after overlay dismissal with either `q` or `Esc`, cancellation while searching exiting 130 with the child terminated and reaped, terminal restoration verified, and stderr replayed where applicable, and a help-only invocation — bare `vrg`, `-h`, and `--help` — printing exactly one help copy to stdout with empty stderr and exit 0, run with ripgrep unavailable on `PATH` and a sentinel fake rg available but never invoked, proving the final binary's no-child/no-TUI startup branch. If any gate or smoke run fails, fix only enough production code to restore the owning issue's composed contract — never patch, weaken, or delete a focused test — and rerun the entire pass until green. Closure fails on any remaining regression; previously green focused tests do not substitute for this clean full pass.

---

### 2. Create the final verification finish marker

**Type**: FINISH MARKER  
**Output**: Finish marker exists at `Notes/finish-markers/035-03/finish-marker.md`.  
**Depends on**: 1

Write `Task 035-03 finished successfully at <time>` to `Notes/finish-markers/035-03/finish-marker.md`, replacing `<time>` with the current UTC timestamp (for example, `date -u +"%Y-%m-%dT%H:%M:%SZ"`). Create the `Notes/finish-markers/035-03/` directory if it does not already exist.

---
