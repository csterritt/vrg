# Issue #50: post-audit re-verification — verify.sh gates, FAKE_RG rename, smoke outcomes

*2026-09-18T12:38:08Z by Showboat 0.6.1*
<!-- showboat-id: b5a497a5-86e7-4913-af6e-3cfca76fac89 -->

Issue #50 is the closing post-audit verification pass over the composed post-audit implementation (Issues #36-#49). Per Notes/issues/050-post-audit-reverification.md, Notes/tasks/050-post-audit-reverification.md, and Notes/PRD-vrg.md (*Testing Decisions*), this walkthrough records: the permanent ten-gate `scripts/verify.sh` run, the explicit uncached and race `cmd/vrg` PTY reruns on the Issue #48 acknowledgement handshakes, the `FAKE_RG_*` fixture-variable rename evidence, and the canonical condition-driven `scripts/smoke.py` run against an untagged production binary covering all five outcomes (browse 0, no-results 1, fatal 2 via `q` and `Esc`, cancellation 130 via `q` and `ctrl+c` with external process-group/EOF/termios observation, help-only 0 with no child). The frozen Issue #35 harness at `Notes/walkthroughs/035-03/code-walkthrough/smoke.py` is unchanged.

The pass caught one real regression: `Child.Terminate` killed only the direct `rg` pid, so a descendant holding the child's output pipes deadlocked the drain-before-`Wait` collection on cancellation. Repaired against the owning Issue #4 terminate-and-reap contract: `spawn` now starts the child a process-group leader and `Terminate`/the context-cancel path kill the whole group (`internal/app/rg.go`, `rg_unix.go`, `rg_other.go`).

```bash
go version; python3 --version; echo '---'; git status --short | head -30; echo '---'; git log --oneline -3
```

```output
go version go1.27.1 linux/arm64
Python 3.14.7
---
 M Notes/wiki/cancellation-cleanup.md
 M Notes/wiki/final-verification.md
 M Notes/wiki/index.md
 M Notes/wiki/log.md
 M Notes/wiki/pty-handshakes.md
 M Notes/wiki/source-code.md
 M Notes/wiki/test-hook-topology.md
 M Notes/wiki/unit-tests.md
 M cmd/vrg/cancel_test.go
 M cmd/vrg/handshake_test.go
 M cmd/vrg/outcome_test.go
 M cmd/vrg/replay_test.go
 M cmd/vrg/runshape_test.go
 M cmd/vrg/search_test.go
 M cmd/vrg/testhooks_test.go
 M internal/app/rg.go
 M internal/app/rg_test.go
?? Notes/walkthroughs/019-04/code-walkthrough/vrg
?? Notes/walkthroughs/020-04/code-walkthrough/vrg
?? Notes/walkthroughs/021-04/code-walkthrough/vrg
?? Notes/walkthroughs/022-04/code-walkthrough/vrg
?? Notes/walkthroughs/023-04/code-walkthrough/vrg
?? Notes/walkthroughs/026-06/code-walkthrough/vrg
?? Notes/walkthroughs/030-04/code-walkthrough/vrg
?? Notes/walkthroughs/031-04/code-walkthrough/vrg
?? Notes/walkthroughs/032-04/code-walkthrough/vrg
?? Notes/walkthroughs/033-04/code-walkthrough/vrg
?? Notes/walkthroughs/034-04/code-walkthrough/vrg
?? Notes/walkthroughs/035-03/code-walkthrough/vrg
?? Notes/walkthroughs/036-06/code-walkthrough/vrg
---
44b8053 Task Notes/tasks/049-tidy-dependency-manifests.md implemented by SWE-2
33a1d93 Task Notes/tasks/049-tidy-dependency-manifests.md implemented by SWE-2
0e09513 Task Notes/tasks/048-pty-tests-deterministic-handshakes.md implemented by SWE-2
```

## Environment and working tree

Go 1.27.1 on linux/arm64, Python 3.14 for the smoke harness. The repository is a detached-head checkout at the Issue #49 head with the Issue #50 changes in the working tree (the `FAKE_RG_*` rename, the process-group termination repair, and this walkthrough/wiki ingest). The untracked `Notes/walkthroughs/*/code-walkthrough/vrg` binaries are pre-existing historical artifacts, untouched by this pass.

## FAKE_RG_* fixture rename

Task 1 renamed every fixture-owned variable off the `VRG_TEST_` prefix. The evidence: no Go file sets or reads the old fixture names (only comments describe the prefix split), and the fake-rg fixtures consume `FAKE_RG_*` names throughout.

```bash
grep -rn 'VRG_TEST_HANDSHAKE\|VRG_TEST_ARGV\|VRG_TEST_CWD\|VRG_TEST_RG_PID\|VRG_TEST_RG_READY\|VRG_TEST_READY\|VRG_TEST_PID' --include='*.go' . ; echo "old fixture-name grep exit: $? (1 = none left)"; echo '---'; grep -rhn 'FAKE_RG_[A-Z_]*' --include='*.go' cmd/vrg internal/app | grep -o 'FAKE_RG_[A-Z_]*' | sort | uniq -c
```

```output
old fixture-name grep exit: 1 (1 = none left)
---
      1 FAKE_RG_
      9 FAKE_RG_ARGV_FILE
      7 FAKE_RG_CWD_FILE
     13 FAKE_RG_HANDSHAKE_FILE
     18 FAKE_RG_PID_FILE
     11 FAKE_RG_READY_FILE
      4 FAKE_RG_SUBPID_FILE
```

## Gate sequence: scripts/verify.sh

The permanent ordered verification entry point runs all ten gates and stops at the first failure. Gates 3-4 keep the `vrg_testhooks` variant under build/vet; gates 5-7 run the suites uncached (`-count=1`), under the race detector, and the `cmd/vrg` PTY suite three times; gates 8-10 are `go mod verify`, pinned `govulncheck@v1.5.0` (network/warm-cache prerequisite; an environment failure exits 75 distinctly), and `go mod tidy -diff`. Output below is the real recorded run.

```bash
scripts/verify.sh; echo "verify.sh exit: $?"
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
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg	95.504s
ok  	vrg/internal/app	2.325s
ok  	vrg/internal/cli	0.006s
ok  	vrg/internal/docs	0.003s
ok  	vrg/internal/filebuffer	0.016s
ok  	vrg/internal/safepresentation	0.003s
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex	0.792s
ok  	vrg/internal/theme	0.005s
ok  	vrg/internal/viewport	0.014s
--- gate 5 OK: go test ./... -count=1 ---

=== gate 6/10: CGO_ENABLED=1 go test -race ./... -count=1 ===
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg	96.827s
ok  	vrg/internal/app	11.386s
ok  	vrg/internal/cli	1.022s
ok  	vrg/internal/docs	1.008s
ok  	vrg/internal/filebuffer	1.028s
ok  	vrg/internal/safepresentation	1.013s
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex	9.049s
ok  	vrg/internal/theme	1.010s
ok  	vrg/internal/viewport	1.031s
--- gate 6 OK: CGO_ENABLED=1 go test -race ./... -count=1 ---

=== gate 7/10: go test ./cmd/vrg -count=3 ===
ok  	vrg/cmd/vrg	285.513s
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
verify.sh exit: 0
```

## Explicit uncached and race PTY reruns

Task 4 requires the Issue #48 handshake suite re-run explicitly beyond the gate sweep: uncached (`-count=1` so nothing stands on a cached result) and under the race detector. `TestNoFixedSleepsInPTYHelpers` runs inside these — the AST-level proof that no fixed settling or inter-key `time.Sleep` remains outside the allow-listed bounded condition polls.

```bash
go test -count=1 ./cmd/vrg; echo "uncached cmd/vrg exit: $?"
```

```output
ok  	vrg/cmd/vrg	95.321s
uncached cmd/vrg exit: 0
```

```bash
CGO_ENABLED=1 go test -race -count=1 ./cmd/vrg; echo "race cmd/vrg exit: $?"
```

```output
ok  	vrg/cmd/vrg	96.736s
race cmd/vrg exit: 0
```

## Smoke outcomes against the untagged production binary

`scripts/smoke.py` is the canonical condition-driven harness — every key send follows an observed UI/output marker, drains happen on process/pipe/PTY-EOF events, a static assertion forbids `time.sleep` in the harness, and smoke environments carry no `VRG_TEST_*` controls (fixture-owned names are `FAKE_RG_*`). The binary under test is built **untagged** — no test seams compiled in — and cancellation is observed externally: child pid and process group gone, PTY EOF, cursor visible, alt-screen exited, termios restored.

```bash
mkdir -p /tmp/vrg-smoke && go build -o /tmp/vrg-smoke/vrg ./cmd/vrg && echo 'untagged build exit: 0' && grep -c 'VRG_TEST_' /tmp/vrg-smoke/vrg | xargs -I{} echo 'VRG_TEST_ byte occurrences in untagged binary: {}'
```

```output
untagged build exit: 0
VRG_TEST_ byte occurrences in untagged binary: 0
```

```bash
python3 scripts/smoke.py; echo "smoke exit: $?"
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
smoke exit: 0
```

```bash
echo '--- env names the smoke harness sets/reads ---'; grep -n 'FAKE_RG_\|VRG_TEST_' scripts/smoke.py; echo '--- frozen Issue #35 harness unchanged ---'; git status --short Notes/walkthroughs/035-03/; git diff --stat Notes/walkthroughs/035-03/; echo "frozen-file diff exit: $? (empty = unchanged)"
```

```output
--- env names the smoke harness sets/reads ---
23:Fixture variables use the FAKE_RG_* namespace (the Issue #50 rename of
24:VRG_TEST_HANDSHAKE/READY/PID/ARGV/CWD). The production smoke runs the
25:untagged binary, so it must neither set nor depend on any VRG_TEST_*
105:    """The production smoke must neither set nor depend on a VRG_TEST_*
108:    leaked = sorted(k for k in env if k.startswith("VRG_TEST_"))
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
--- frozen Issue #35 harness unchanged ---
?? Notes/walkthroughs/035-03/code-walkthrough/vrg
frozen-file diff exit: 0 (empty = unchanged)
```

## Outcome

All ten `scripts/verify.sh` gates green; the explicit uncached and race `cmd/vrg` PTY reruns green on the Issue #48 handshakes; all five smoke outcomes verified against the untagged production binary — browse 0, no-results 1, fatal 2 under `q` and `Esc` with the composed exit-code/integrity/record-loss diagnostics replayed, cancellation 130 under `q` and `ctrl+c` with the child pid, process group, PTY EOF, and terminal state all externally observed, and help-only 0 with exactly one usage copy and no child. The one regression this pass surfaced — descendants surviving a direct-pid kill and deadlocking cancellation — was repaired against the owning Issue #4 contract by making the spawned child a process-group leader and killing the whole group, covered by `TestTerminateKillsChildProcessGroup` and `TestCancelTerminatesChildProcessGroup`. The smoke environment carries only `FAKE_RG_*` fixture names; no `VRG_TEST_*` control reaches the production binary. The frozen Issue #35 harness and its transcript are unchanged. Wiki ingestion: [post-audit-verification.md](../../../wiki/post-audit-verification.md).

