# Issue #50: Post-audit re-verification — full suite, race, repeated PTY runs, and tidy/vuln gates

*2026-09-25T15:06:32Z by Showboat 0.6.1*
<!-- showboat-id: eab0f95d-d887-4c92-9cda-af463c20ae44 -->

Walkthrough for Issue #50's closing task (Notes/tasks/050-post-audit-reverification.md, parent PRD Notes/PRD-vrg.md *Testing Decisions*): the post-audit re-verification pass over the composed Issues #36–#49 implementation. Task 1 renamed the fixture-owned fake-rg variables out of the VRG_* namespace — the active fixtures' consolidated capture variable VRG_CAPTURE_DIR became FAKE_RG_CAPTURE_DIR and the help-sentinel's VRG_RG_MARKER became FAKE_RG_MARKER — keeping fixture names disjoint from the vrg-consumed VRG_TEST_* seam manifest; the canonical condition-driven smoke harness lives at scripts/smoke.py on the renamed FAKE_RG_* contract (the Issue #35 walkthrough artifact is a frozen historical record, not rewritten). Task 3's permanent entry point scripts/verify.sh runs ten ordered gates and fails on the first non-zero step; its pinned govulncheck gate documents the network/warm-cache prerequisite and reports an unmet one as an environment failure (exit 75), not a repository regression. The Issue #50 task names predate the Issue #48 handshake refactor, so each named contract is re-run under its current carrier test (mapped inline below). Every gate runs from a clean detached-HEAD worktree checkout of the post-rename commit so no stale artifact can mask a failure.

```bash
cd /tmp/vrg-50-verify && git log --oneline -2 && git status --porcelain -uno | wc -l | xargs -I{} echo 'uncommitted tracked modifications: {}' && echo '== FAKE_RG_* rename state ==' && grep -rn 'VRG_CAPTURE_DIR\|VRG_RG_MARKER' cmd/vrg/ || echo 'no VRG_CAPTURE_DIR/VRG_RG_MARKER remain in cmd/vrg'
```

```output
d722f86 Issue #50: fix ready-file create/write race in internal/app pid polls
139cc81 Issue #50 Task 1: rename fixture-owned variables to FAKE_RG_*
uncommitted tracked modifications: 0
== FAKE_RG_* rename state ==
no VRG_CAPTURE_DIR/VRG_RG_MARKER remain in cmd/vrg
```

Gate set: scripts/verify.sh from the clean checkout — build, vet, the tagged variants, the uncached suite, the race suite, three consecutive cmd/vrg runs, module verification, the pinned govulncheck@v1.5.0 scan, and the fail-closed tidy gate. Test durations are stripped so the document verifies cleanly; the exit status is reported explicitly.

```bash
cd /tmp/vrg-50-verify && ./scripts/verify.sh 2>&1 | sed -E 's/\t[0-9]+\.[0-9]+s$//'; echo "verify.sh -> exit ${PIPESTATUS[0]}"
```

```output

=== gate 1/10: go build ./... ===
--- gate 1 OK: go build ./... ---

=== gate 2/10: go vet ./... ===
--- gate 2 OK: go vet ./... ---

=== gate 3/10: go build -tags vrg_testhooks ./cmd/vrg ===
--- gate 3 OK: go build -tags vrg_testhooks ./cmd/vrg ---

=== gate 4/10: go vet -tags vrg_testhooks ./cmd/vrg ===
--- gate 4 OK: go vet -tags vrg_testhooks ./cmd/vrg ---

=== gate 5/10: go test ./... -count=1 ===
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
--- gate 5 OK: go test ./... -count=1 ---

=== gate 6/10: CGO_ENABLED=1 go test -race ./... -count=1 ===
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
--- gate 6 OK: CGO_ENABLED=1 go test -race ./... -count=1 ---

=== gate 7/10: go test ./cmd/vrg -count=3 ===
ok  	vrg/cmd/vrg
--- gate 7 OK: go test ./cmd/vrg -count=3 ---

=== gate 8/10: go mod verify ===
all modules verified
--- gate 8 OK: go mod verify ---

=== gate 9/10: go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./... ===
No vulnerabilities found.
--- gate 9 OK: govulncheck ---

=== gate 10/10: go mod tidy -diff ===
--- gate 10 OK: go mod tidy -diff ---

=== verify.sh: all 10 gates green ===
verify.sh -> exit 0
```

Explicit uncached PTY/subprocess rerun of the Issue #50 named contracts under their Issue #48 carrier names — outcome (fatal overlay naming the exit status over usable results, the fatal no-results session inside the handshake matrix, warning-overlay dismissal), replay (ctrl+c / q while searching / q during gate-held preparation / normal q after a completed warning stream / controlled failure alongside an earlier diagnostic / embedded-newline-and-ESC filename), and cancellation/boundary (q and ctrl+c against a blocked fake rg, ordinary exit reaping, gate-held cancellation, injected controlled failure, child argv/workdir, start failure, dual-pipe backpressure, stderr fixture). The signal-death naming contract is covered at the outcome level by internal/app's sigErr fixture tests, which gate 5 ran uncached.

```bash
cd /tmp/vrg-50-verify && go test -count=1 -v ./cmd/vrg -run '^(TestPTYNonZeroExitBrowseOverlayExits2|TestHandshakeMatrixRowsAcknowledged|TestOverlayDismissalAcknowledgedBeforeQuit|TestPTYQuitAfterCompletedStreamReplaysWarning|TestPTYStderrContentFixture|TestPTYCtrlCAfterDiagnosticReplaysOnce|TestPTYQWhileSearchingReplaysDiagnostic|TestPTYQDuringGateHeldPreparationReplaysDiagnostic|TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics|TestPTYReplayEscapesEmbeddedFilename|TestPTYQWhileSearchingExits130|TestPTYCtrlCWhileSearchingExits130|TestPTYSIGINTWhileSearchingExits130|TestPTYOrdinaryExitReapsChild|TestPTYQDuringGateHeldPreparationExits130|TestPTYControlledFailureExits2|TestControlledFailureDiagnosticOnStderr|TestSearchLifecycleAtBoundary|TestStartFailureNoRipgrep|TestDualPipeDrainageAtBoundary)$' 2>&1 | grep -E '^(--- (PASS|FAIL|SKIP)|ok|FAIL|PASS)' | sed -E 's/[[:space:]]*\([0-9]+\.[0-9]+s\)//g; s/\t[0-9]+\.[0-9]+s$//'; echo "uncached named rerun -> exit ${PIPESTATUS[0]}"
```

```output
--- PASS: TestHandshakeMatrixRowsAcknowledged
--- PASS: TestOverlayDismissalAcknowledgedBeforeQuit
--- PASS: TestSearchLifecycleAtBoundary
--- PASS: TestDualPipeDrainageAtBoundary
--- PASS: TestStartFailureNoRipgrep
--- PASS: TestPTYCtrlCAfterDiagnosticReplaysOnce
--- PASS: TestPTYQWhileSearchingReplaysDiagnostic
--- PASS: TestPTYQDuringGateHeldPreparationReplaysDiagnostic
--- PASS: TestPTYQuitAfterCompletedStreamReplaysWarning
--- PASS: TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics
--- PASS: TestPTYReplayEscapesEmbeddedFilename
--- PASS: TestPTYQWhileSearchingExits130
--- PASS: TestPTYCtrlCWhileSearchingExits130
--- PASS: TestPTYSIGINTWhileSearchingExits130
--- PASS: TestPTYQDuringGateHeldPreparationExits130
--- PASS: TestPTYOrdinaryExitReapsChild
--- PASS: TestPTYControlledFailureExits2
--- PASS: TestControlledFailureDiagnosticOnStderr
--- PASS: TestPTYNonZeroExitBrowseOverlayExits2
--- PASS: TestPTYStderrContentFixture
PASS
ok  	vrg/cmd/vrg
uncached named rerun -> exit 0
```

```bash
cd /tmp/vrg-50-verify && CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg 2>&1 | sed -E 's/\t[0-9]+\.[0-9]+s$//'; echo "race rerun -> exit ${PIPESTATUS[0]}"
```

```output
ok  	vrg/cmd/vrg
race rerun -> exit 0
```

The five smoke outcomes via the canonical scripts/smoke.py against the untagged production binary (distinct from the vrg_testhooks binary the subprocess tests build): browse 0, no-results 1, fatal 2 under both q and Esc with the composed integrity/record-loss diagnostics, cancellation 130 with the child and its process group externally observed gone plus PTY EOF and terminal restoration, and help-only 0 with exactly one help copy and a sentinel rg never invoked. Every key is gated on an observed rendered marker — the harness carries no time.sleep and proves it by parsing its own AST.

```bash
cd /tmp/vrg-50-verify && go build -o /tmp/vrg-smoke/vrg ./cmd/vrg && python3 scripts/smoke.py; echo "smoke.py -> exit $?"
```

```output
Scenario 0: successful browse, q exits 0
  [PASS] browse exit 0
  [PASS] browse view shows test.txt
  [PASS] browse stderr empty
  [PASS] browse PTY reached EOF
Scenario 1: no-results search, warning dismissed, q exits 1
  [PASS] no-results exit 1
  [PASS] no-results shows 'No results found'
  [PASS] no-results stderr replays 'warn'
  [PASS] no-results PTY reached EOF
Scenario 2: fatal fake-rg, composed integrity/record-loss diagnostics, q and Esc exit 2
  [PASS] fatal q exit 2
  [PASS] fatal q overlay names exit code
  [PASS] fatal q overlay states integrity cause
  [PASS] fatal q overlay states record-loss cause
  [PASS] fatal q stderr replays composed diagnostic
  [PASS] fatal Esc exit 2
Scenario 130: cancellation while searching, child and process group gone, terminal restored
  [PASS] cancel exit 130
  [PASS] child process gone
  [PASS] child process group gone
  [PASS] cancel PTY reached EOF
  [PASS] terminal cursor restored
  [PASS] alt screen exited
  [PASS] termios restored
Scenario 130b: cancellation via ctrl+c while searching
  [PASS] ctrl+c exit 130
  [PASS] ctrl+c child process gone
  [PASS] ctrl+c process group gone
Scenario help-only: bare vrg, -h, --help (sentinel fake rg never invoked)
  [PASS] bare vrg exit 0
  [PASS] bare vrg exactly one 'Usage:' on stdout
  [PASS] bare vrg stderr empty
  [PASS] bare vrg no terminal control sequences
  [PASS] -h exit 0
  [PASS] -h exactly one 'Usage:' on stdout
  [PASS] -h stderr empty
  [PASS] -h no terminal control sequences
  [PASS] --help exit 0
  [PASS] --help exactly one 'Usage:' on stdout
  [PASS] --help stderr empty
  [PASS] --help no terminal control sequences
  [PASS] sentinel fake rg never invoked

SMOKE OK: all five outcomes verified
smoke.py -> exit 0
```

```bash
cd /tmp/vrg-50-verify && echo '== env names the smoke harness sets ==' && grep -oE '"(FAKE_RG|VRG_TEST)_[A-Z_]+"' scripts/smoke.py | sort -u && echo '== env names the cmd/vrg fixtures set ==' && grep -rhoE '"(FAKE_RG|VRG_TEST)_[A-Z_]+=' cmd/vrg/*_test.go | sort -u && echo '== fixture shell scripts consume ==' && grep -rhoE 'FAKE_RG_[A-Z_]+' cmd/vrg/*_test.go scripts/smoke.py | sort | uniq -c
```

```output
== env names the smoke harness sets ==
"FAKE_RG_HANDSHAKE_FILE"
"FAKE_RG_PID_FILE"
"FAKE_RG_READY_FILE"
== env names the cmd/vrg fixtures set ==
"FAKE_RG_CAPTURE_DIR=
"FAKE_RG_MARKER=
"VRG_TEST_COLLECT_ACK=
"VRG_TEST_DIAGNOSTIC_TEXT=
"VRG_TEST_DIAGNOSTIC_TRIGGER=
"VRG_TEST_EVENT_ACK=
"VRG_TEST_FAIL_DIAGNOSTIC=
"VRG_TEST_FAIL_TRIGGER=
"VRG_TEST_GATE=
"VRG_TEST_REAP=
"VRG_TEST_RUN_ERROR=
"VRG_TEST_RUN_FINAL_MODEL=
== fixture shell scripts consume ==
     17 FAKE_RG_CAPTURE_DIR
      9 FAKE_RG_HANDSHAKE_FILE
      1 FAKE_RG_MARKER
      6 FAKE_RG_PID_FILE
      6 FAKE_RG_READY_FILE
```

The smoke environment carries only FAKE_RG_* fixture names (HANDSHAKE_FILE/READY_FILE/PID_FILE) and no VRG_TEST_* controls — assert_no_vrg_test_env makes a leaked name a hard failure; the cmd/vrg environment lists set FAKE_RG_CAPTURE_DIR/FAKE_RG_MARKER for the fixtures while every VRG_TEST_* name remains a vrg-consumed tagged seam in Issue #45's explicit hook manifest.

All gates green on the clean checkout of the post-rename tree: scripts/verify.sh exits 0 across all ten ordered gates including the race suite, three consecutive cmd/vrg runs, the pinned govulncheck scan (no vulnerabilities), and a clean go mod tidy -diff; the uncached and race PTY/subprocess reruns pass every Issue #50 named contract under its current Issue #48 carrier name on the acknowledgement handshakes, with no fixed settling or inter-key delay outside the bounded-poll allowlist TestHarnessHasNoFixedDelays enforces; and the five smoke outcomes through canonical scripts/smoke.py hold against the untagged production binary — browse 0, no-results 1 with the warning replayed, fatal 2 under q and Esc carrying the composed 'rg failed: exit status 2' + 'missing summary' + malformed-record diagnostics, cancellation 130 under both q and ctrl+c with the child and its process group externally observed gone, PTY EOF, cursor/alt-screen/termios restoration, and help-only 0 with one Usage copy and the sentinel rg never invoked. Cross-references: Notes/PRD-vrg.md *Testing Decisions*, Notes/wiki/final-verification.md (the Issue #35 analogue), Notes/wiki/pty-handshake-harness.md, Notes/wiki/test-hook-build-topology.md.

Re-executing this document surfaced one real harness-side race, which the pass then repaired: internal/app's awaitReadyPID/awaitPIDFile read the fake child's ready/grandchild handshake file the moment it existed — os.WriteFile creates the file before the pid bytes land, so an existent-but-empty read failed strconv.Atoi inside TestCancelTerminatesAndReapsChild (a create/write race in the test helper, not a production regression; no behavioral assertion weakened). Both helpers now require non-empty content, the same semantics cmd/vrg's awaitFileContent already carries (commit d722f86). The proof is a 10-run recorded stress of the cancellation tests (a 30-run pre-check also passed), then the whole closing pass re-runs below on the post-fix commit.

```bash
cd /tmp/vrg-50-verify && git log --oneline -1 && go test -count=10 -run 'TestCancel' ./internal/app 2>&1 | sed -E 's/\t[0-9]+\.[0-9]+s$//'; echo "cancel stress -> exit ${PIPESTATUS[0]}"
```

```output
d722f86 Issue #50: fix ready-file create/write race in internal/app pid polls
ok  	vrg/internal/app
cancel stress -> exit 0
```

Final closing pass on the post-fix commit (d722f86): the full ten-gate verify.sh, then the named PTY/subprocess contracts uncached and under race, then the five smoke outcomes rebuilt and re-run.

```bash
cd /tmp/vrg-50-verify && ./scripts/verify.sh 2>&1 | sed -E 's/\t[0-9]+\.[0-9]+s$//'; echo "verify.sh -> exit ${PIPESTATUS[0]}"
```

```output

=== gate 1/10: go build ./... ===
--- gate 1 OK: go build ./... ---

=== gate 2/10: go vet ./... ===
--- gate 2 OK: go vet ./... ---

=== gate 3/10: go build -tags vrg_testhooks ./cmd/vrg ===
--- gate 3 OK: go build -tags vrg_testhooks ./cmd/vrg ---

=== gate 4/10: go vet -tags vrg_testhooks ./cmd/vrg ===
--- gate 4 OK: go vet -tags vrg_testhooks ./cmd/vrg ---

=== gate 5/10: go test ./... -count=1 ===
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
--- gate 5 OK: go test ./... -count=1 ---

=== gate 6/10: CGO_ENABLED=1 go test -race ./... -count=1 ===
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
--- gate 6 OK: CGO_ENABLED=1 go test -race ./... -count=1 ---

=== gate 7/10: go test ./cmd/vrg -count=3 ===
ok  	vrg/cmd/vrg
--- gate 7 OK: go test ./cmd/vrg -count=3 ---

=== gate 8/10: go mod verify ===
all modules verified
--- gate 8 OK: go mod verify ---

=== gate 9/10: go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./... ===
No vulnerabilities found.
--- gate 9 OK: govulncheck ---

=== gate 10/10: go mod tidy -diff ===
--- gate 10 OK: go mod tidy -diff ---

=== verify.sh: all 10 gates green ===
verify.sh -> exit 0
```

```bash
cd /tmp/vrg-50-verify && go test -count=1 -v ./cmd/vrg -run '^(TestPTYNonZeroExitBrowseOverlayExits2|TestHandshakeMatrixRowsAcknowledged|TestOverlayDismissalAcknowledgedBeforeQuit|TestPTYQuitAfterCompletedStreamReplaysWarning|TestPTYStderrContentFixture|TestPTYCtrlCAfterDiagnosticReplaysOnce|TestPTYQWhileSearchingReplaysDiagnostic|TestPTYQDuringGateHeldPreparationReplaysDiagnostic|TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics|TestPTYReplayEscapesEmbeddedFilename|TestPTYQWhileSearchingExits130|TestPTYCtrlCWhileSearchingExits130|TestPTYSIGINTWhileSearchingExits130|TestPTYOrdinaryExitReapsChild|TestPTYQDuringGateHeldPreparationExits130|TestPTYControlledFailureExits2|TestControlledFailureDiagnosticOnStderr|TestSearchLifecycleAtBoundary|TestStartFailureNoRipgrep|TestDualPipeDrainageAtBoundary)$' 2>&1 | grep -E '^(--- (PASS|FAIL|SKIP)|ok|FAIL|PASS)' | sed -E 's/[[:space:]]*\([0-9]+\.[0-9]+s\)//g; s/\t[0-9]+\.[0-9]+s$//' && CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg 2>&1 | sed -E 's/\t[0-9]+\.[0-9]+s$//'; echo "pty reruns -> exit ${PIPESTATUS[0]}"
```

```output
--- PASS: TestHandshakeMatrixRowsAcknowledged
--- PASS: TestOverlayDismissalAcknowledgedBeforeQuit
--- PASS: TestSearchLifecycleAtBoundary
--- PASS: TestDualPipeDrainageAtBoundary
--- PASS: TestStartFailureNoRipgrep
--- PASS: TestPTYCtrlCAfterDiagnosticReplaysOnce
--- PASS: TestPTYQWhileSearchingReplaysDiagnostic
--- PASS: TestPTYQDuringGateHeldPreparationReplaysDiagnostic
--- PASS: TestPTYQuitAfterCompletedStreamReplaysWarning
--- PASS: TestPTYControlledFailureReplaysAlongsideEarlierDiagnostics
--- PASS: TestPTYReplayEscapesEmbeddedFilename
--- PASS: TestPTYQWhileSearchingExits130
--- PASS: TestPTYCtrlCWhileSearchingExits130
--- PASS: TestPTYSIGINTWhileSearchingExits130
--- PASS: TestPTYQDuringGateHeldPreparationExits130
--- PASS: TestPTYOrdinaryExitReapsChild
--- PASS: TestPTYControlledFailureExits2
--- PASS: TestControlledFailureDiagnosticOnStderr
--- PASS: TestPTYNonZeroExitBrowseOverlayExits2
--- PASS: TestPTYStderrContentFixture
PASS
ok  	vrg/cmd/vrg
ok  	vrg/cmd/vrg
pty reruns -> exit 0
```

```bash
cd /tmp/vrg-50-verify && go build -o /tmp/vrg-smoke/vrg ./cmd/vrg && python3 scripts/smoke.py; echo "smoke.py -> exit $?"
```

```output
Scenario 0: successful browse, q exits 0
  [PASS] browse exit 0
  [PASS] browse view shows test.txt
  [PASS] browse stderr empty
  [PASS] browse PTY reached EOF
Scenario 1: no-results search, warning dismissed, q exits 1
  [PASS] no-results exit 1
  [PASS] no-results shows 'No results found'
  [PASS] no-results stderr replays 'warn'
  [PASS] no-results PTY reached EOF
Scenario 2: fatal fake-rg, composed integrity/record-loss diagnostics, q and Esc exit 2
  [PASS] fatal q exit 2
  [PASS] fatal q overlay names exit code
  [PASS] fatal q overlay states integrity cause
  [PASS] fatal q overlay states record-loss cause
  [PASS] fatal q stderr replays composed diagnostic
  [PASS] fatal Esc exit 2
Scenario 130: cancellation while searching, child and process group gone, terminal restored
  [PASS] cancel exit 130
  [PASS] child process gone
  [PASS] child process group gone
  [PASS] cancel PTY reached EOF
  [PASS] terminal cursor restored
  [PASS] alt screen exited
  [PASS] termios restored
Scenario 130b: cancellation via ctrl+c while searching
  [PASS] ctrl+c exit 130
  [PASS] ctrl+c child process gone
  [PASS] ctrl+c process group gone
Scenario help-only: bare vrg, -h, --help (sentinel fake rg never invoked)
  [PASS] bare vrg exit 0
  [PASS] bare vrg exactly one 'Usage:' on stdout
  [PASS] bare vrg stderr empty
  [PASS] bare vrg no terminal control sequences
  [PASS] -h exit 0
  [PASS] -h exactly one 'Usage:' on stdout
  [PASS] -h stderr empty
  [PASS] -h no terminal control sequences
  [PASS] --help exit 0
  [PASS] --help exactly one 'Usage:' on stdout
  [PASS] --help stderr empty
  [PASS] --help no terminal control sequences
  [PASS] sentinel fake rg never invoked

SMOKE OK: all five outcomes verified
smoke.py -> exit 0
```

Closing state: on the post-fix commit the ten-gate scripts/verify.sh exits 0 from the clean checkout, the named PTY/subprocess contracts pass uncached and under the race detector on the Issue #48 handshakes, and the five smoke outcomes hold via canonical scripts/smoke.py against the untagged production binary. The one finding — the internal/app ready-file create/write race — was a test-harness repair (commit d722f86), not a production regression; every audit-repair contract from Issues #36–#49 held under the composed run. This walkthrough is the closing review evidence for Issue #50; the pass is ingested in Notes/wiki/post-audit-verification.md and recorded in Notes/wiki/log.md.
