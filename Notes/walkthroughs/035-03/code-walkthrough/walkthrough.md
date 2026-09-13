# Issue #35: Final integration verification — clean build, vet, test, and smoke run

*2026-09-12T15:06:47Z by Showboat 0.6.1*
<!-- showboat-id: 3750f26f-245d-4780-aabc-5c9cef796138 -->

Walkthrough for Issue #35 (Notes/tasks/035-final-integration-verification.md), the final integration verification task. This is the closing pass for the whole vrg implementation: a clean-checkout repository-wide build, vet, and test pass; an explicit no-cache rerun of the PTY/subprocess boundary suites from Issues #4, #9, and #11; and a final-binary smoke run through the five representative outcomes with the Issue #4 fake-rg harness. References: Notes/PRD-vrg.md (Testing Decisions, Outcome and exit-status contract), Issue #35.

Clean checkout preparation: clear the Go build and test caches so no stale artifact can mask a failure.

```bash
cd /home/chris/vrg && go clean -cache && go clean -testcache && echo CACHES-CLEANED
```

```output
CACHES-CLEANED
```

Gate 1: go build ./... over the fully composed implementation.

```bash
cd /home/chris/vrg && go build ./... && echo BUILD-OK; echo BUILD_EXIT=$?
```

```output
BUILD-OK
BUILD_EXIT=0
```

Gate 2: go vet ./... over the fully composed implementation.

```bash
cd /home/chris/vrg && go vet ./... && echo VET-OK; echo VET_EXIT=$?
```

```output
VET-OK
VET_EXIT=0
```

Gate 3: go test ./... over the fully composed implementation.

```bash
cd /home/chris/vrg && go test ./... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'; echo TEST_EXIT=${PIPESTATUS[0]}
```

```output
?   	vrg/Notes/walkthroughs/023-04/code-walkthrough/demo_artifacts	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/docs
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
TEST_EXIT=0
```

Explicit no-cache rerun of the PTY/subprocess boundary suites from Issues #4, #9, and #11 (all in cmd/vrg), with -count=1 so every critical boundary test executes rather than being satisfied by a cached result.

```bash
cd /home/chris/vrg && go test -count=1 -v ./cmd/vrg/ -timeout 120s 2>&1 | sed 's/[[:space:]][0-9.]*s$//'; echo PTY_EXIT=${PIPESTATUS[0]}
```

```output
=== RUN   TestQAgainstBlockedFakeRGExits130
--- PASS: TestQAgainstBlockedFakeRGExits130 (0.03s)
=== RUN   TestCtrlCAgainstBlockedFakeRGExits130
--- PASS: TestCtrlCAgainstBlockedFakeRGExits130 (0.03s)
=== RUN   TestNormalExitReapsChild
--- PASS: TestNormalExitReapsChild (0.03s)
=== RUN   TestQDuringGateHeldPreparationExits130
--- PASS: TestQDuringGateHeldPreparationExits130 (0.03s)
=== RUN   TestInjectedControlledFailure
--- PASS: TestInjectedControlledFailure (0.03s)
=== RUN   TestGeneratedHelpStdout
=== RUN   TestGeneratedHelpStdout/#00
=== RUN   TestGeneratedHelpStdout/-h
=== RUN   TestGeneratedHelpStdout/--help
=== RUN   TestGeneratedHelpStdout/foo_--help_/nonexistent
=== RUN   TestGeneratedHelpStdout/-h_foo_bar_baz
=== RUN   TestGeneratedHelpStdout/foo_bar_baz_--help
=== RUN   TestGeneratedHelpStdout/-i_--help
=== RUN   TestGeneratedHelpStdout/-ih
=== RUN   TestGeneratedHelpStdout/-ih_foo
=== RUN   TestGeneratedHelpStdout/--ignore-case_--help
=== RUN   TestGeneratedHelpStdout/foo_src_--help
=== RUN   TestGeneratedHelpStdout/-uuu_--help
=== RUN   TestGeneratedHelpStdout/--ignore-case=false_--help
=== RUN   TestGeneratedHelpStdout/--unsupported_--help
=== RUN   TestGeneratedHelpStdout/foo_/nonexistent_--help
--- PASS: TestGeneratedHelpStdout (0.34s)
    --- PASS: TestGeneratedHelpStdout/#00 (0.02s)
    --- PASS: TestGeneratedHelpStdout/-h (0.02s)
    --- PASS: TestGeneratedHelpStdout/--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/foo_--help_/nonexistent (0.02s)
    --- PASS: TestGeneratedHelpStdout/-h_foo_bar_baz (0.02s)
    --- PASS: TestGeneratedHelpStdout/foo_bar_baz_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/-i_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/-ih (0.02s)
    --- PASS: TestGeneratedHelpStdout/-ih_foo (0.02s)
    --- PASS: TestGeneratedHelpStdout/--ignore-case_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/foo_src_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/-uuu_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/--ignore-case=false_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/--unsupported_--help (0.02s)
    --- PASS: TestGeneratedHelpStdout/foo_/nonexistent_--help (0.02s)
=== RUN   TestHelpWithoutRipgrep
--- PASS: TestHelpWithoutRipgrep (0.09s)
=== RUN   TestHelpDoesNotInvokeRipgrep
--- PASS: TestHelpDoesNotInvokeRipgrep (0.02s)
=== RUN   TestHelpIgnoresExecutableName
--- PASS: TestHelpIgnoresExecutableName (0.02s)
=== RUN   TestCLIOutputSafety
--- PASS: TestCLIOutputSafety (0.07s)
=== RUN   TestExecutableBoundary
=== RUN   TestExecutableBoundary/--
=== RUN   TestExecutableBoundary/a_b_c
=== RUN   TestExecutableBoundary/--unsupported
=== RUN   TestExecutableBoundary/foo_-e
=== RUN   TestExecutableBoundary/foo_/nonexistent-vrg-root
=== RUN   TestExecutableBoundary/foo_-
=== RUN   TestExecutableBoundary/foo_/dev/null
=== RUN   TestExecutableBoundary/-i
--- PASS: TestExecutableBoundary (0.20s)
    --- PASS: TestExecutableBoundary/-- (0.02s)
    --- PASS: TestExecutableBoundary/a_b_c (0.02s)
    --- PASS: TestExecutableBoundary/--unsupported (0.02s)
    --- PASS: TestExecutableBoundary/foo_-e (0.02s)
    --- PASS: TestExecutableBoundary/foo_/nonexistent-vrg-root (0.02s)
    --- PASS: TestExecutableBoundary/foo_- (0.02s)
    --- PASS: TestExecutableBoundary/foo_/dev/null (0.02s)
    --- PASS: TestExecutableBoundary/-i (0.02s)
=== RUN   TestDashFileRootAtProcessBoundary
--- PASS: TestDashFileRootAtProcessBoundary (0.34s)
=== RUN   TestHelpAssignmentSpellingsAreNotHelp
--- PASS: TestHelpAssignmentSpellingsAreNotHelp (0.07s)
=== RUN   TestFlagContractUsageErrors
=== RUN   TestFlagContractUsageErrors/-e_foo
=== RUN   TestFlagContractUsageErrors/--type_go_foo
=== RUN   TestFlagContractUsageErrors/--max-count=3_foo
=== RUN   TestFlagContractUsageErrors/-uuu_foo
=== RUN   TestFlagContractUsageErrors/-u_-uu_foo
=== RUN   TestFlagContractUsageErrors/-iuuu_foo
=== RUN   TestFlagContractUsageErrors/-u_--unrestricted_-u_foo
=== RUN   TestFlagContractUsageErrors/--unrestricted_-uu_foo
=== RUN   TestFlagContractUsageErrors/--ignore-case=false_foo
=== RUN   TestFlagContractUsageErrors/-i=false_foo
=== RUN   TestFlagContractUsageErrors/--unrestricted=false_foo
=== RUN   TestFlagContractUsageErrors/--help=false_foo
=== RUN   TestFlagContractUsageErrors/-h=false_foo
=== RUN   TestFlagContractUsageErrors/-foo
=== RUN   TestFlagContractUsageErrors/-i
--- PASS: TestFlagContractUsageErrors (0.34s)
    --- PASS: TestFlagContractUsageErrors/-e_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/--type_go_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/--max-count=3_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/-uuu_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/-u_-uu_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/-iuuu_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/-u_--unrestricted_-u_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/--unrestricted_-uu_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/--ignore-case=false_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/-i=false_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/--unrestricted=false_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/--help=false_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/-h=false_foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/-foo (0.02s)
    --- PASS: TestFlagContractUsageErrors/-i (0.02s)
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/OSC
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/CSI
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/C0
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/C1
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/DEL
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/StandaloneCR
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/InvalidUTF8
=== RUN   TestSinkSafetyTableUsageErrorProcessBoundary/EmbeddedNewline
--- PASS: TestSinkSafetyTableUsageErrorProcessBoundary (0.18s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/OSC (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/CSI (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/C0 (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/C1 (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/DEL (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/StandaloneCR (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/InvalidUTF8 (0.02s)
    --- PASS: TestSinkSafetyTableUsageErrorProcessBoundary/EmbeddedNewline (0.02s)
=== RUN   TestSinkSafetyTableHelpStdoutProcessBoundary
--- PASS: TestSinkSafetyTableHelpStdoutProcessBoundary (0.02s)
=== RUN   TestFatalExitWithResultsShowsOverlay
--- PASS: TestFatalExitWithResultsShowsOverlay (0.24s)
=== RUN   TestFatalExitNoOutputNamesExitCode
--- PASS: TestFatalExitNoOutputNamesExitCode (0.14s)
=== RUN   TestFatalExitNoOutputEscExits2
--- PASS: TestFatalExitNoOutputEscExits2 (0.19s)
=== RUN   TestSignalDeathNamesSignal
--- PASS: TestSignalDeathNamesSignal (1.04s)
=== RUN   TestStderrWarningWithSummaryShowsWarningOverlay
--- PASS: TestStderrWarningWithSummaryShowsWarningOverlay (0.26s)
=== RUN   TestStderrContentFixture
--- PASS: TestStderrContentFixture (0.28s)
=== RUN   TestReplayCtrlCAfterStderrDiagnostic
--- PASS: TestReplayCtrlCAfterStderrDiagnostic (0.13s)
=== RUN   TestReplayQWhileSearchingAfterDiagnostic
--- PASS: TestReplayQWhileSearchingAfterDiagnostic (0.14s)
=== RUN   TestReplayQWhileGateHeldAfterDiagnostic
--- PASS: TestReplayQWhileGateHeldAfterDiagnostic (0.14s)
=== RUN   TestReplayNormalQAfterCompletedStreamWithWarning
--- PASS: TestReplayNormalQAfterCompletedStreamWithWarning (0.34s)
=== RUN   TestReplayControlledFailureWithEarlierDiagnostic
--- PASS: TestReplayControlledFailureWithEarlierDiagnostic (0.05s)
=== RUN   TestReplayFilenameWithNewlineAndESC
--- PASS: TestReplayFilenameWithNewlineAndESC (0.14s)
=== RUN   TestChildArgvAndWorkdir
--- PASS: TestChildArgvAndWorkdir (0.23s)
=== RUN   TestChildArgvWithFlags
--- PASS: TestChildArgvWithFlags (0.24s)
=== RUN   TestStartFailureExit2
--- PASS: TestStartFailureExit2 (0.03s)
=== RUN   TestStartFailureExplicitPath
--- PASS: TestStartFailureExplicitPath (0.02s)
=== RUN   TestDualPipeBackpressure
--- PASS: TestDualPipeBackpressure (0.37s)
=== RUN   TestStderrCapturedWithoutBlocking
--- PASS: TestStderrCapturedWithoutBlocking (0.33s)
PASS
ok  	vrg/cmd/vrg
PTY_EXIT=0
```

Build the final vrg binary for the smoke run.

```bash
cd /home/chris/vrg && go build -o /tmp/vrg-smoke/vrg ./cmd/vrg && echo BINARY-BUILT && /tmp/vrg-smoke/vrg --help | head -3
```

```output
BINARY-BUILT
Usage: vrg [OPTIONS] PATTERN [ROOT]

Search with ripgrep and browse the results in a terminal UI.
```

Final-binary smoke run through the five representative outcomes with the Issue #4 fake-rg harness under a real PTY: (1) successful browse with q exiting 0; (2) no-results search with q exiting 1 and the collected warn diagnostic replayed to stderr; (3) fatal fake-rg with no usable results exiting 2 on dismissal with both q and Esc; (4) cancellation while searching exiting 130 with the child terminated and reaped, terminal restoration verified (cursor-show, alt-screen exit, termios restored), for both q and ctrl+c; (5) help-only invocation (bare vrg, -h, --help) printing exactly one help copy to stdout with empty stderr and exit 0, run with ripgrep unavailable on PATH and a sentinel fake rg available but never invoked.

```bash
cd /home/chris/vrg && timeout 120 python3 /tmp/vrg-smoke/smoke.py; echo SMOKE_EXIT=$?
```

```output
Scenario 0: successful browse, q exits 0
  [PASS] browse exit 0
  [PASS] browse view shows test.txt
  [PASS] browse stderr empty
Scenario 1: no-results search, q exits 1
  [PASS] no-results exit 1
  [PASS] no-results shows 'No results found'
  [PASS] no-results stderr replays 'warn'
Scenario 2: fatal fake-rg no usable results, q exits 2
  [PASS] fatal q exit 2
  [PASS] fatal q overlay names exit code
Scenario 2b: fatal fake-rg no usable results, Esc exits 2
  [PASS] fatal Esc exit 2
Scenario 130: cancellation while searching, child reaped, terminal restored
  [PASS] cancel exit 130
  [PASS] child terminated/reaped
  [PASS] reap evidence present
  [PASS] terminal cursor restored
  [PASS] alt screen exited
  [PASS] termios restored
Scenario 130b: cancellation via ctrl+c while searching
  [PASS] ctrl+c exit 130
Scenario help-only: bare vrg, -h, --help (ripgrep unavailable, sentinel fake rg never invoked)
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
SMOKE_EXIT=0
```

All gates and smoke outcomes green on the first clean pass. No regressions were found; no production code was changed; no focused test was patched, weakened, or deleted. The clean-checkout build/vet/test pass, the explicit no-cache PTY/subprocess rerun, and the five final-binary smoke outcomes together close Issue #35 and the whole vrg implementation. See Notes/wiki/final-verification.md for the wiki record.
