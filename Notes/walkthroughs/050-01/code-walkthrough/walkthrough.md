# Issue #50: post-audit re-verification

*2026-09-15T23:47:07Z by Showboat 0.6.1*
<!-- showboat-id: ddd2c49c-e291-4ff9-9123-46b02c2e41e5 -->

Walkthrough for Issue #50 (Notes/tasks/050-post-audit-reverification.md, Notes/issues/050-post-audit-reverification.md) — the closing verification pass over the composed post-audit implementation (Issues #36-49), the audit cycle counterpart to Issue #35. The audit overall assessment (Notes/critiques/final-audit-vrg.md) required rerunning the complete acceptance and PTY verification suite with deterministic handshakes; this pass proves the fixes compose with no regression. References: Notes/PRD-vrg.md (*Testing Decisions* — subprocess boundary: deterministic application-side handshakes, bounded condition-poll sleeps only); Notes/wiki/post-audit-reverification.md.

Recorded below:
- The permanent gate runner scripts/verify.sh executed from the checkout: ten ordered gates, every gate output and exit status.
- The explicit uncached and race-mode reruns of the named PTY/subprocess tests on the Issue #48 handshakes.
- The five smoke scenarios run via the canonical condition-driven harness scripts/smoke.py against the untagged production binary (built by go build -o /tmp/vrg-smoke/vrg ./cmd/vrg), distinct from the vrg_testhooks binary the subprocess tests use.
- Evidence that fake-rg fixture variables use only FAKE_RG_* names and that the smoke environment carries no VRG_TEST_* controls.
- The FAKE_RG_* fixture rename across the active cmd/vrg PTY fixtures.

## Gate 0-10: scripts/verify.sh from the checkout

The permanent Issue #50 gate runner executes ten ordered gates and fails on the first non-zero step: untagged build and vet, the vrg_testhooks build/vet variants, the uncached full suite, the race-mode full suite, the repeated cmd/vrg PTY run, go mod verify, the pinned govulncheck@v1.5.0 invocation, and the go mod tidy -diff no-drift gate (the permanent tidy gate Issue #49 refers to). The govulncheck gate requires network access or pre-populated module/tool and vulnerability-database caches; an offline clean machine without them is reported as an environment failure (exit 75), not a repository regression.

```bash
cd /home/chris/vrg && ./scripts/verify.sh; echo "verify.sh exit=$?"
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
?   	vrg/Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts	[no test files]
ok  	vrg/cmd/vrg	5.835s
ok  	vrg/internal/app	12.094s
ok  	vrg/internal/cli	0.005s
ok  	vrg/internal/docs	0.026s
ok  	vrg/internal/filebuffer	0.012s
ok  	vrg/internal/safepresentation	0.002s
ok  	vrg/internal/searchindex	3.233s
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme	0.001s
ok  	vrg/internal/viewport	0.007s
--- gate 5 OK: go test ./... -count=1 ---

=== gate 6/10: CGO_ENABLED=1 go test -race ./... -count=1 ===
?   	vrg/Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts	[no test files]
ok  	vrg/cmd/vrg	7.437s
ok  	vrg/internal/app	67.055s
ok  	vrg/internal/cli	1.021s
ok  	vrg/internal/docs	1.268s
ok  	vrg/internal/filebuffer	1.026s
ok  	vrg/internal/safepresentation	1.012s
ok  	vrg/internal/searchindex	31.114s
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme	1.008s
ok  	vrg/internal/viewport	1.020s
--- gate 6 OK: CGO_ENABLED=1 go test -race ./... -count=1 ---

=== gate 7/10: go test ./cmd/vrg -count=3 ===
ok  	vrg/cmd/vrg	16.624s
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
verify.sh exit=0
```

## Explicit PTY/subprocess reruns — uncached

The named outcome, replay, and cancellation/boundary tests rerun explicitly and uncached on the Issue #48 handshakes. No fixed settling or inter-key time.Sleep synchronization remains in runVrgWithKeys, runVrgKillChild, runVrgWithQuit, runVrgCancel, or any runVrgReplay trigger callback — TestNoFixedSleepsInPtyHelpers (included below) confines every sleep to the named bounded condition polls.

```bash
cd /home/chris/vrg && go test -count=1 -v ./cmd/vrg -run 'Test(FatalExitWithResultsShowsOverlay|FatalExitNoOutputNamesExitCode|FatalExitNoOutputEscExits2|SignalDeathNamesSignal|StderrWarningWithSummaryShowsWarningOverlay|StderrContentFixture|ReplayCtrlCAfterStderrDiagnostic|ReplayQWhileSearchingAfterDiagnostic|ReplayQWhileGateHeldAfterDiagnostic|ReplayNormalQAfterCompletedStreamWithWarning|ReplayControlledFailureWithEarlierDiagnostic|ReplayFilenameWithNewlineAndESC|QAgainstBlockedFakeRGExits130|CtrlCAgainstBlockedFakeRGExits130|NormalExitReapsChild|QDuringGateHeldPreparationExits130|InjectedControlledFailure|ChildArgvAndWorkdir|StartFailureExit2|DualPipeBackpressure|StderrCapturedWithoutBlocking|NoFixedSleepsInPtyHelpers)$' 2>&1 | sed 's/([0-9.]*s)//g'; echo "uncached named-test run exit=${PIPESTATUS[0]}"
```

```output
=== RUN   TestQAgainstBlockedFakeRGExits130
--- PASS: TestQAgainstBlockedFakeRGExits130 
=== RUN   TestCtrlCAgainstBlockedFakeRGExits130
--- PASS: TestCtrlCAgainstBlockedFakeRGExits130 
=== RUN   TestNormalExitReapsChild
--- PASS: TestNormalExitReapsChild 
=== RUN   TestQDuringGateHeldPreparationExits130
--- PASS: TestQDuringGateHeldPreparationExits130 
=== RUN   TestInjectedControlledFailure
--- PASS: TestInjectedControlledFailure 
=== RUN   TestNoFixedSleepsInPtyHelpers
--- PASS: TestNoFixedSleepsInPtyHelpers 
=== RUN   TestFatalExitWithResultsShowsOverlay
--- PASS: TestFatalExitWithResultsShowsOverlay 
=== RUN   TestFatalExitNoOutputNamesExitCode
--- PASS: TestFatalExitNoOutputNamesExitCode 
=== RUN   TestFatalExitNoOutputEscExits2
--- PASS: TestFatalExitNoOutputEscExits2 
=== RUN   TestSignalDeathNamesSignal
--- PASS: TestSignalDeathNamesSignal 
=== RUN   TestStderrWarningWithSummaryShowsWarningOverlay
--- PASS: TestStderrWarningWithSummaryShowsWarningOverlay 
=== RUN   TestStderrContentFixture
--- PASS: TestStderrContentFixture 
=== RUN   TestReplayCtrlCAfterStderrDiagnostic
--- PASS: TestReplayCtrlCAfterStderrDiagnostic 
=== RUN   TestReplayQWhileSearchingAfterDiagnostic
--- PASS: TestReplayQWhileSearchingAfterDiagnostic 
=== RUN   TestReplayQWhileGateHeldAfterDiagnostic
--- PASS: TestReplayQWhileGateHeldAfterDiagnostic 
=== RUN   TestReplayNormalQAfterCompletedStreamWithWarning
--- PASS: TestReplayNormalQAfterCompletedStreamWithWarning 
=== RUN   TestReplayControlledFailureWithEarlierDiagnostic
--- PASS: TestReplayControlledFailureWithEarlierDiagnostic 
=== RUN   TestReplayFilenameWithNewlineAndESC
--- PASS: TestReplayFilenameWithNewlineAndESC 
=== RUN   TestChildArgvAndWorkdir
--- PASS: TestChildArgvAndWorkdir 
=== RUN   TestStartFailureExit2
--- PASS: TestStartFailureExit2 
=== RUN   TestDualPipeBackpressure
--- PASS: TestDualPipeBackpressure 
=== RUN   TestStderrCapturedWithoutBlocking
--- PASS: TestStderrCapturedWithoutBlocking 
PASS
ok  	vrg/cmd/vrg	2.441s
uncached named-test run exit=0
```

## Explicit PTY/subprocess reruns — race mode

The same named tests under the race detector.

```bash
cd /home/chris/vrg && CGO_ENABLED=1 go test -race -count=1 -v ./cmd/vrg -run 'Test(FatalExitWithResultsShowsOverlay|FatalExitNoOutputNamesExitCode|FatalExitNoOutputEscExits2|SignalDeathNamesSignal|StderrWarningWithSummaryShowsWarningOverlay|StderrContentFixture|ReplayCtrlCAfterStderrDiagnostic|ReplayQWhileSearchingAfterDiagnostic|ReplayQWhileGateHeldAfterDiagnostic|ReplayNormalQAfterCompletedStreamWithWarning|ReplayControlledFailureWithEarlierDiagnostic|ReplayFilenameWithNewlineAndESC|QAgainstBlockedFakeRGExits130|CtrlCAgainstBlockedFakeRGExits130|NormalExitReapsChild|QDuringGateHeldPreparationExits130|InjectedControlledFailure|ChildArgvAndWorkdir|StartFailureExit2|DualPipeBackpressure|StderrCapturedWithoutBlocking)$' 2>&1 | sed 's/([0-9.]*s)//g'; echo "race named-test run exit=${PIPESTATUS[0]}"
```

```output
=== RUN   TestQAgainstBlockedFakeRGExits130
--- PASS: TestQAgainstBlockedFakeRGExits130 
=== RUN   TestCtrlCAgainstBlockedFakeRGExits130
--- PASS: TestCtrlCAgainstBlockedFakeRGExits130 
=== RUN   TestNormalExitReapsChild
--- PASS: TestNormalExitReapsChild 
=== RUN   TestQDuringGateHeldPreparationExits130
--- PASS: TestQDuringGateHeldPreparationExits130 
=== RUN   TestInjectedControlledFailure
--- PASS: TestInjectedControlledFailure 
=== RUN   TestFatalExitWithResultsShowsOverlay
--- PASS: TestFatalExitWithResultsShowsOverlay 
=== RUN   TestFatalExitNoOutputNamesExitCode
--- PASS: TestFatalExitNoOutputNamesExitCode 
=== RUN   TestFatalExitNoOutputEscExits2
--- PASS: TestFatalExitNoOutputEscExits2 
=== RUN   TestSignalDeathNamesSignal
--- PASS: TestSignalDeathNamesSignal 
=== RUN   TestStderrWarningWithSummaryShowsWarningOverlay
--- PASS: TestStderrWarningWithSummaryShowsWarningOverlay 
=== RUN   TestStderrContentFixture
--- PASS: TestStderrContentFixture 
=== RUN   TestReplayCtrlCAfterStderrDiagnostic
--- PASS: TestReplayCtrlCAfterStderrDiagnostic 
=== RUN   TestReplayQWhileSearchingAfterDiagnostic
--- PASS: TestReplayQWhileSearchingAfterDiagnostic 
=== RUN   TestReplayQWhileGateHeldAfterDiagnostic
--- PASS: TestReplayQWhileGateHeldAfterDiagnostic 
=== RUN   TestReplayNormalQAfterCompletedStreamWithWarning
--- PASS: TestReplayNormalQAfterCompletedStreamWithWarning 
=== RUN   TestReplayControlledFailureWithEarlierDiagnostic
--- PASS: TestReplayControlledFailureWithEarlierDiagnostic 
=== RUN   TestReplayFilenameWithNewlineAndESC
--- PASS: TestReplayFilenameWithNewlineAndESC 
=== RUN   TestChildArgvAndWorkdir
--- PASS: TestChildArgvAndWorkdir 
=== RUN   TestStartFailureExit2
--- PASS: TestStartFailureExit2 
=== RUN   TestDualPipeBackpressure
--- PASS: TestDualPipeBackpressure 
=== RUN   TestStderrCapturedWithoutBlocking
--- PASS: TestStderrCapturedWithoutBlocking 
PASS
ok  	vrg/cmd/vrg	3.767s
race named-test run exit=0
```

## The five smoke scenarios — canonical harness, untagged production binary

The untagged production binary (distinct from the vrg_testhooks binary the subprocess tests use) is built exactly as Issue #50 prescribes, then driven by the canonical condition-driven harness scripts/smoke.py. Every key follows an observed rendered marker; draining is completion/EOF-driven; cancellation cleanup is proven externally (kill-0 process and process-group probes) with no VRG_TEST_* name in the environment. The fatal fixture emits a malformed record and exits 2, so the overlay and stderr replay carry the composed Issues #36-37/#44 diagnostics: the process line naming the exit code, the missing-summary integrity cause, and the malformed record-loss component.

```bash
cd /home/chris/vrg && go build -o /tmp/vrg-smoke/vrg ./cmd/vrg && python3 scripts/smoke.py; echo "smoke exit=$?"
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
smoke exit=0
```

## FAKE_RG_* rename evidence

The fake-rg-owned variables moved off the VRG_TEST_ prefix in every active PTY fixture and test env list: VRG_TEST_ARGV/CWD/HANDSHAKE/READY/PID became FAKE_RG_ARGV_FILE/FAKE_RG_CWD_FILE/FAKE_RG_HANDSHAKE_FILE/FAKE_RG_READY_FILE/FAKE_RG_PID_FILE. The old names no longer appear in Go sources; the remaining VRG_TEST_* names are only the explicit vrg-consumed hook manifest (VRG_TEST_REAP, VRG_TEST_GATE, VRG_TEST_FAIL_*, VRG_TEST_DIAGNOSTIC_*, VRG_TEST_COLLECT_ACK, VRG_TEST_UPDATE_ACK, VRG_TEST_RUN_*), consumed by the tagged seams, not the fixtures.

```bash
cd /home/chris/vrg && echo "--- old fixture names in Go sources ---" && (grep -rn "VRG_TEST_\(ARGV\|CWD\|HANDSHAKE\|READY\|PID\)" --include="*.go" . ; echo "grep exit=$? (1 = none remain)") && echo "--- FAKE_RG_* usage counts per file ---" && grep -rc "FAKE_RG_" cmd/vrg/*_test.go && echo "--- remaining VRG_TEST_* names (seam manifest only) ---" && grep -rhoE "VRG_TEST_[A-Z_]+" cmd/vrg --include="*.go" | sort -u
```

```output
--- old fixture names in Go sources ---
grep exit=1 (1 = none remain)
--- FAKE_RG_* usage counts per file ---
cmd/vrg/cancel_test.go:25
cmd/vrg/handshake_test.go:0
cmd/vrg/main_test.go:0
cmd/vrg/outcome_test.go:5
cmd/vrg/replay_test.go:10
cmd/vrg/runshape_test.go:4
cmd/vrg/search_test.go:13
cmd/vrg/testhooks_test.go:3
--- remaining VRG_TEST_* names (seam manifest only) ---
VRG_TEST_COLLECT_ACK
VRG_TEST_DIAGNOSTIC_TEXT
VRG_TEST_DIAGNOSTIC_TRIGGER
VRG_TEST_FAIL_DIAGNOSTIC
VRG_TEST_FAIL_TRIGGER
VRG_TEST_GATE
VRG_TEST_REAP
VRG_TEST_RUN_ERROR
VRG_TEST_RUN_FINAL_MODEL
VRG_TEST_UPDATE_ACK
```

## Smoke harness contract evidence

The canonical scripts/smoke.py uses only FAKE_RG_* fixture names; every VRG_TEST match in the file belongs to assert_no_vrg_test_env, the guard that fails any run whose environment carries a VRG_TEST_* name — so the production smoke provably neither sets nor depends on the test seams (cancellation cleanup is observed via external kill-0/process-group probes, not VRG_TEST_REAP). The harness contains no time.sleep call and no other fixed delay used as a progress proxy: the AST self-check (assert_no_fixed_delays) parses the file on every startup and fails on any time.sleep call, and the only waits are bounded condition polls re-checking explicit conditions. The frozen Issue #35 artifact at Notes/walkthroughs/035-03/code-walkthrough/smoke.py is unchanged.

```bash
cd /home/chris/vrg && echo "--- FAKE_RG_* names in scripts/smoke.py ---" && grep -n "FAKE_RG_" scripts/smoke.py && echo "--- every VRG_TEST match (guard only) ---" && grep -n "VRG_TEST" scripts/smoke.py && echo "--- AST check: no time.sleep call in scripts/smoke.py ---" && python3 -c "import ast,sys
tree=ast.parse(open(\"scripts/smoke.py\").read())
bad=[n for n in ast.walk(tree) if isinstance(n,ast.Call) and isinstance(n.func,ast.Attribute) and n.func.attr==\"sleep\" and isinstance(n.func.value,ast.Name) and n.func.value.id==\"time\"]
print(\"time.sleep calls:\",len(bad)); sys.exit(1 if bad else 0)" && echo "AST check: PASS (no time.sleep calls)" && echo "--- frozen Issue #35 artifact unchanged ---" && git -C /home/chris/vrg status --porcelain Notes/walkthroughs/035-03/ && echo "git status for 035-03 clean (no output above)"
```

```output
--- FAKE_RG_* names in scripts/smoke.py ---
23:Fixture variables use the FAKE_RG_* namespace (the Issue #50 rename of
374:if [ -n "$FAKE_RG_HANDSHAKE_FILE" ]; then touch "$FAKE_RG_HANDSHAKE_FILE"; fi
379:                      "FAKE_RG_HANDSHAKE_FILE": hs},
414:if [ -n "$FAKE_RG_HANDSHAKE_FILE" ]; then touch "$FAKE_RG_HANDSHAKE_FILE"; fi
419:                      "FAKE_RG_HANDSHAKE_FILE": hs},
463:if [ -n "$FAKE_RG_HANDSHAKE_FILE" ]; then touch "$FAKE_RG_HANDSHAKE_FILE"; fi
470:                          "FAKE_RG_HANDSHAKE_FILE": hs},
517:if [ -n "$FAKE_RG_PID_FILE" ]; then echo $$ > "$FAKE_RG_PID_FILE"; fi
518:if [ -n "$FAKE_RG_READY_FILE" ]; then touch "$FAKE_RG_READY_FILE"; fi
523:                      "FAKE_RG_READY_FILE": ready,
524:                      "FAKE_RG_PID_FILE": pid_file},
574:if [ -n "$FAKE_RG_PID_FILE" ]; then echo $$ > "$FAKE_RG_PID_FILE"; fi
575:if [ -n "$FAKE_RG_READY_FILE" ]; then touch "$FAKE_RG_READY_FILE"; fi
580:                      "FAKE_RG_READY_FILE": ready,
581:                      "FAKE_RG_PID_FILE": pid_file},
--- every VRG_TEST match (guard only) ---
24:VRG_TEST_HANDSHAKE/READY/PID/ARGV/CWD). The production smoke runs the
25:untagged binary, so it must neither set nor depend on any VRG_TEST_*
105:    """The production smoke must neither set nor depend on a VRG_TEST_*
108:    leaked = sorted(k for k in env if k.startswith("VRG_TEST_"))
--- AST check: no time.sleep call in scripts/smoke.py ---
time.sleep calls: 0
AST check: PASS (no time.sleep calls)
--- frozen Issue #35 artifact unchanged ---
git status for 035-03 clean (no output above)
```

## Summary

Issue #50 closing pass is complete and green:
- scripts/verify.sh is the permanent verification entry point: all ten ordered gates green from the checkout — untagged and vrg_testhooks build/vet variants, uncached and race full-suite runs, the repeated cmd/vrg PTY run, go mod verify, pinned govulncheck@v1.5.0 (no reachable vulnerabilities; its network/cache prerequisite documented as an environment failure mode, exit 75), and go mod tidy -diff with zero drift.
- The named outcome, replay, and cancellation/boundary tests pass uncached and under race on the Issue #48 handshakes; TestNoFixedSleepsInPtyHelpers confines every sleep to the named bounded condition polls.
- Fake-rg fixture variables use FAKE_RG_* names throughout the active PTY fixtures and test env lists; VRG_TEST_* names remain only in the explicit vrg-consumed hook manifest consumed by the tagged seams.
- The five smoke scenarios ran via the canonical condition-driven scripts/smoke.py against the untagged production binary: browse 0, no-results 1, fatal 2 under both q and Esc with the composed integrity/record-loss diagnostics (process line, missing-summary integrity cause, malformed record-loss component — in the overlay and the stderr replay), cancellation 130 with externally observed child/process-group termination, PTY EOF, and terminal restoration, and help-only 0 with exactly one Usage copy and the sentinel fake rg never invoked. The smoke environment carries no VRG_TEST_* controls.
- The Issue #35 smoke copy at Notes/walkthroughs/035-03/code-walkthrough/smoke.py remains a frozen historical artifact.
- Regressions: none in repository code; one defect inside the new scripts/smoke.py harness itself (a missing index increment in strip_ansi) was found and fixed during bring-up.

References: Issue #50 (Notes/issues/050-post-audit-reverification.md, Notes/tasks/050-post-audit-reverification.md), Notes/PRD-vrg.md (*Testing Decisions*), Notes/critiques/final-audit-vrg.md (Overall assessment), Notes/wiki/post-audit-reverification.md.
