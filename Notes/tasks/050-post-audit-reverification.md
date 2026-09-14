# Tasks for #50: Post-audit re-verification — full suite, race, repeated PTY runs, and tidy/vuln gates

Parent issue: #50
Parent PRD: PRD-vrg.md
**Blocked by issues**: #36, #37, #38, #39, #40, #41, #42, #43, #44, #45, #46, #47, #48, #49
**Acceptance criteria**: AC5 → Tasks 1, 4; AC6 → Task 2; AC1, AC3–AC4 → Tasks 3–4; AC2, AC7 → Task 4; AC8 → Task 6
**Manual verification**: Task 6 owns the issue's manual checks.

## Tasks

### 1. Rename fixture-owned variables to `FAKE_RG_*`

**Type**: REFACTOR  
**Output**: `VRG_TEST_HANDSHAKE`, `VRG_TEST_READY`, `VRG_TEST_PID`, `VRG_TEST_ARGV`, and `VRG_TEST_CWD` are renamed to corresponding `FAKE_RG_*` names throughout the PTY/smoke fixtures, the renamed fixtures drive the suite unchanged, and the production smoke neither sets nor depends on any `VRG_TEST_*` name.  
**Depends on**: none

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Begin only after Issues #36–#49 are complete — this is the closing pass over the composed post-audit implementation. Rename the fixture-owned fake-rg variables off the `VRG_TEST_` prefix in every PTY and smoke fixture — `VRG_TEST_HANDSHAKE`, `VRG_TEST_READY`, `VRG_TEST_PID`, `VRG_TEST_ARGV`, and `VRG_TEST_CWD` become corresponding `FAKE_RG_*` names (for example `FAKE_RG_PID_FILE`) — in the fake-rg shell fixtures and the `cmd/vrg` test environment lists that set them. Names consumed by the tagged vrg acknowledgement seams stay `VRG_TEST_*` and remain in Issue #45's explicit hook manifest; only fake-rg-owned variables are renamed. The existing uncached `cmd/vrg` suite is the unchanged behavioural safety net — run it before and after the rename, and add an assertion or review check that the production smoke environment contains no `VRG_TEST_*` name.

---

### 2. Make the smoke harness condition-driven

**Type**: REFACTOR  
**Output**: `Notes/walkthroughs/035-03/code-walkthrough/smoke.py` sends every key only after the preceding expected UI state or output marker is observed, drains on completion/EOF, uses bounded condition polls only for explicit conditions, and contains no `time.sleep` or other fixed settling delay used as a progress proxy — proven by a static/review assertion and a green run.  
**Depends on**: 1

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

Modernize the fake-rg PTY smoke harness at `Notes/walkthroughs/035-03/code-walkthrough/smoke.py`: replace every fixed pre-key, inter-key, process-exit, and post-exit sleep with bounded waits for externally observable PTY output/state markers, process or pipe completion, and EOF; make draining completion/EOF-driven. A fake-child ready file may prove the fixture started, but it is not sufficient evidence that the app entered a state or processed a prior key — send every key only after the preceding expected UI state/output is observed. For cancellation, the fake rg fixture may receive the renamed `FAKE_RG_*` variables and expose its PID/process group while the parent harness externally observes that the process and process group are gone, PTY completion/EOF has occurred, and terminal state is restored. Add a static/review assertion that the harness contains no `time.sleep` call and no other fixed delay used as a progress proxy. Run the harness against a locally built binary to prove it stays green.

---

### 3. Create the permanent verification entry point

**Type**: CONFIG  
**Output**: Executable `scripts/verify.sh` runs the ten ordered gates from a clean checkout and fails on the first non-zero step, with the govulncheck network/cache prerequisite documented.  
**Depends on**: 1

Create `scripts/verify.sh` — executable, runnable from a clean checkout with the declared tool/network prerequisites — running exactly this ordered gate set and failing on the first non-zero step: `go build ./...`; `go vet ./...`; `go build -tags vrg_testhooks ./cmd/vrg`; `go vet -tags vrg_testhooks ./cmd/vrg`; `go test ./... -count=1`; `CGO_ENABLED=1 go test -race ./... -count=1`; `go test ./cmd/vrg -count=3`; `go mod verify`; `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`; and `go mod tidy -diff` — the permanent tidy gate Issue #49 refers to, which must report no drift. Document that the pinned govulncheck step requires network access to fetch the tool and vulnerability database, or pre-populated module/tool and vulnerability-database caches, and that an offline clean machine without those caches must be reported as an environment failure, not a repository regression. Validate the script's syntax and run it end to end.

---

### 4. Run the closing verification pass

**Type**: GREEN  
**Output**: From a clean checkout, `scripts/verify.sh` exits 0; the named PTY/subprocess tests pass uncached and under race on Issue #48 handshakes with no fixed settling delay in any helper; and the five smoke scenarios against the untagged production binary produce their contracted exit statuses with terminal restoration, correct stderr replay, and externally observed cancellation cleanup.  
**Depends on**: 2, 3

Before changing code, read and follow the coding standards in `Notes/skills/AGENTS.md`.

From a clean checkout with the documented prerequisites, run `scripts/verify.sh` to green. Separately re-run the critical PTY/subprocess tests explicitly and uncached — `go test -count=1 ./cmd/vrg` and `CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg` — covering at minimum the issue's named outcome tests (`TestFatalExitWithResultsShowsOverlay`, `TestFatalExitNoOutputNamesExitCode`, `TestFatalExitNoOutputEscExits2`, `TestSignalDeathNamesSignal`, `TestStderrWarningWithSummaryShowsWarningOverlay`, `TestStderrContentFixture`), replay tests (`TestReplayCtrlCAfterStderrDiagnostic`, `TestReplayQWhileSearchingAfterDiagnostic`, `TestReplayQWhileGateHeldAfterDiagnostic`, `TestReplayNormalQAfterCompletedStreamWithWarning`, `TestReplayControlledFailureWithEarlierDiagnostic`, `TestReplayFilenameWithNewlineAndESC`), and cancellation/boundary tests (`TestQAgainstBlockedFakeRGExits130`, `TestCtrlCAgainstBlockedFakeRGExits130`, `TestNormalExitReapsChild`, `TestQDuringGateHeldPreparationExits130`, `TestInjectedControlledFailure`, `TestChildArgvAndWorkdir`, `TestStartFailureExit2`, `TestDualPipeBackpressure`, `TestStderrCapturedWithoutBlocking`), all operating on Issue #48's handshakes. Confirm no fixed settling or inter-key `time.Sleep` synchronization remains in `runVrgWithKeys`, `runVrgKillChild`, `runVrgWithQuit`, `runVrgCancel`, or any `runVrgReplay` trigger callback; bounded condition-poll sleeps are permissible only when each iteration checks an explicit condition. Run the five smoke scenarios with the modernized harness against the **untagged** production binary built by `go build -o /tmp/vrg-smoke/vrg ./cmd/vrg` — distinct from the `vrg_testhooks`-tagged binary used by the subprocess tests: successful browse → 0; no-results → 1; fatal → 2 under both `q` and `Esc` dismissal with the newly composed fatal diagnostics (Issues #36–#37, #44) stating the integrity/record-loss causes; cancellation → 130 with the child and its process group externally observed gone, PTY EOF, and the terminal restored; and help-only → 0 (bare `vrg`, `-h`, `--help`; exactly one help copy on stdout, empty stderr, sentinel fake rg never invoked) — with the smoke environment setting and depending on no `VRG_TEST_*` name. Fix any regression against the owning issue's contract — never by weakening or deleting a focused test — and rerun the entire pass until green.

---

### 5. Document the post-audit verification pass

**Type**: DOCUMENT  
**Output**: Wiki documentation records `scripts/verify.sh`, its ordered gates and prerequisites, the explicit PTY reruns, the smoke outcomes, and any regressions found and repaired.  
**Depends on**: 4

Read and follow `Notes/wiki/wiki-rules.md` and the schema in `Notes/wiki/AGENTS.md`, then ingest the completed Issue #50 verification pass into the appropriate pages under `Notes/wiki`. Document `scripts/verify.sh` and each gate including the tagged build/vet variants and the govulncheck prerequisite, the `FAKE_RG_*` fixture rename, the condition-driven smoke harness, the uncached and race PTY runs on the Issue #48 handshakes, the five smoke outcomes against the untagged production binary including the composed fatal diagnostics, and any regressions with the owning issue whose contract was restored. Cross-reference Issue #50, the *Testing Decisions* section of `Notes/PRD-vrg.md`, and the audit's overall assessment in `Notes/critiques/final-audit-vrg.md`, update `Notes/wiki/index.md`, and append the required dated ingest record to `Notes/wiki/log.md` without rewriting previous entries.

---

### 6. Create the post-audit closing walkthrough

**Type**: CODE WALKTHROUGH  
**Output**: Showboat walkthrough exists at `Notes/walkthroughs/050-01/code-walkthrough` recording every command, output, and exit status as the closing review evidence for the audit cycle.  
**Depends on**: 5

Use showboat, consulting `uvx showboat --help`, to create the walkthrough at exactly `Notes/walkthroughs/050-01/code-walkthrough` — the path is fixed by Issue #50, not derived from the task ordinal — with the main file named `walkthrough.md`. Record the complete closing pass: the clean-checkout `scripts/verify.sh` run with every gate's output and exit status, the explicit uncached and race PTY runs of the named tests, and the five smoke scenarios against the untagged production binary — browse 0, no-results 1, fatal 2 under both `q` and `Esc` with the composed integrity/record-loss diagnostics, cancellation 130 with externally observed process-group termination, PTY EOF, and terminal restoration, and help-only 0 with exactly one help copy on stdout and no child — including evidence that the smoke environment uses only `FAKE_RG_*` fixture names and no `VRG_TEST_*` controls. Reference Issue #50 and `Notes/PRD-vrg.md`, and store every generated artifact in the approved directory.

---
