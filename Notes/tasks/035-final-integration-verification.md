# Tasks for #35: Final integration verification — clean repository-wide build, vet, test, and smoke run

Parent issue: #35
Parent PRD: PRD-vrg.md
**Blocked by issues**: #30, #33, #34
**Acceptance criteria**: AC1–AC4 → Task 1; AC5 → Task 3
**Manual verification**: Task 3 owns the issue's manual checks.

## Tasks

### 1. Run the clean repository-wide verification pass

**Type**: GREEN  
**Output**: From a clean checkout, `go build ./...`, `go vet ./...`, and `go test ./...` pass over the fully composed implementation with the PTY/subprocess tests explicitly executed, and the final binary's four smoke scenarios (browse 0, no results 1, fatal 2, cancellation 130) succeed; any regression is repaired against the owning issue's contract and the full suite reruns green.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #30, #33, and #34 are complete — they are the dependency graph's terminal issues and together transitively require every feature issue. The composed RED contracts of Issues #1–#34 are this task's contract. Start from a clean checkout of the completed repository (a fresh clone or fully cleaned worktree) so no stale artifact can mask a failure. Run `go build ./...`, `go vet ./...`, and `go test ./...`, then explicitly re-run the PTY/subprocess test packages from Issues #4, #9, and #11 with test caching disabled (for example `-count=1`) so every critical boundary test executes rather than being satisfied by a cached result, without sleeps. Build the final `vrg` binary and smoke-run it through the four representative outcomes with the Issue #4 fake-rg harness: a successful browse exiting 0 after `q`, a no-results search exiting 1 after `q`, a fatal fake-rg outcome with no usable results exiting 2 after overlay dismissal with either `q` or `Esc`, and cancellation while searching exiting 130 with the child terminated and reaped, terminal restoration verified, and stderr replayed where applicable. If any gate or smoke run fails, fix only enough production code to restore the owning issue's composed contract — never patch, weaken, or delete a focused test — and rerun the entire pass until green. Closure fails on any remaining regression; previously green focused tests do not substitute for this clean full pass.

---

### 2. Document the final verification pass

**Type**: DOCUMENT  
**Output**: Wiki documentation records the clean-checkout gate results, the explicitly executed PTY/subprocess suites, the four smoke outcomes, and any regressions found and repaired.  
**Depends on**: 1

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #35 verification pass into the appropriate pages under `Notes/wiki`. Document the clean-checkout `go build ./...`, `go vet ./...`, and `go test ./...` results, the explicit no-cache rerun of the PTY/subprocess suites, the four final-binary smoke outcomes with their exit statuses, terminal restoration, and stderr replay, and any regressions the pass uncovered together with the owning issue whose contract was restored. Cross-reference Issue #35 and the Testing Decisions section of `Notes/PRD-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 3. Create the final verification walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/035-03/code-walkthrough`.  
**Depends on**: 2

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/035-03/code-walkthrough`, with the main file named `walkthrough.md`. Demonstrate and record the complete closing pass: the clean-checkout `go build ./...`, `go vet ./...`, and `go test ./...` runs with their outputs, the explicitly executed PTY/subprocess tests, and the final binary's four smoke scenarios — a successful browse with `q` exiting 0, a no-results search exiting 1, a fatal fake-rg outcome with no usable results exiting 2 after dismissal with both `q` and `Esc`, and cancellation while searching exiting 130 with the reaped child, restored terminal, and replayed diagnostics — capturing every command, its output, and its exit status as the review evidence for the whole task set. Reference Issue #35 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
