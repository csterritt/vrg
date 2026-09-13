## Issue 50: Post-audit re-verification — full suite, race, repeated PTY runs, and tidy/vuln gates

**Type**: AFK
**Blocked by**: Issues 36–49 — this is the closing pass over the composed post-audit implementation, mirroring Issue 35's role for the original set

### Parent PRD

`Notes/PRD-vrg.md` — audit source: `Notes/critiques/final-audit-vrg.md`, Overall assessment ("rerun the complete acceptance and PTY verification suite with deterministic handshakes")

### What to build

The single closing verification pass over the repository after all audit-fix issues have landed — the counterpart to Issue 35 for the audit cycle. No new product behavior is built here; this issue owns the proof that the fixes compose, no regression was introduced, and the gates the audit ran are green on the final state.

- From a clean checkout of the completed repository, run the full gate set the audit performed: `go build ./...`, `go vet ./...`, `go test ./... -count=1`, `CGO_ENABLED=1 go test -race ./... -count=1`, `go test ./cmd/vrg -count=3`, `go mod verify`, `govulncheck ./...` (pinned version), and `go mod tidy -diff` — which must now report no drift per Issue 49.
- Re-run the critical PTY/subprocess tests explicitly and uncached; they must now operate on the Issue 48 handshakes with no fixed sleeps.
- Re-run the representative smoke scenarios from Issue 35 (browse → 0, no-results → 1, fatal → 2, cancel → 130, help-only → 0 with no child/TUI) to confirm the audit fixes did not disturb the outcome contract — with special attention to the newly composed fatal diagnostics (Issues 36–37, 44) appearing correctly in overlays and stderr replay.
- Record every command and its result in a post-audit walkthrough artifact as the closing review evidence for the audit cycle.

A regression discovered by this pass is fixed against the owning issue's contract — never by weakening or deleting a focused test.

### How to verify

- **Manual**: run each gate command from a clean checkout and record outputs/exit statuses; run the smoke scenarios with the fake-rg harness and confirm exit statuses, terminal restoration, and — for fatal outcomes — that the composed diagnostics now state the integrity/record-loss causes.
- **Automated**: `go test ./...` is the pass; PTY/subprocess packages re-run with `-count=1` (and `-race`) so no cached result substitutes; `go mod tidy -diff` is empty; `govulncheck` reports no vulnerabilities.

### Acceptance criteria

- [ ] Given a clean checkout after Issues 36–49, then `go build ./...`, `go vet ./...`, `go test ./... -count=1`, and the race-enabled suite all pass.
- [ ] Given the PTY/subprocess tests, then they pass uncached and with deterministic handshakes — no fixed sleeps.
- [ ] Given `go mod tidy -diff`, then it reports a clean manifest.
- [ ] Given `govulncheck ./...` at the pinned version, then no reachable vulnerabilities are reported.
- [ ] Given the five smoke scenarios, then each produces its contracted exit status with terminal restoration and correct stderr replay — including the improved fatal-outcome explanations.
- [ ] Given any regression found, then it is fixed against the owning issue's contract and the full suite rerun green.
- [ ] Given the completed pass, then all commands and results are recorded in the post-audit walkthrough artifact.

### User stories addressed

- User stories 1–85: the composed post-audit implementation is proven to build, vet, test, and run end to end, with the audit's high-severity gaps closed.

---
