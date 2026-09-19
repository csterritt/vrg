# Post-audit re-verification (Issue #50)

Delivered by
[Issue #50](../issues/050-post-audit-reverification.md)
([task](../tasks/050-post-audit-reverification.md)): the closing
verification pass over the composed post-audit implementation —
Issues #36–#49 repaired the audit findings, and this pass proves the
repairs compose without regressions. The audit source is
`Notes/critiques/final-audit-vrg.md` (not present in this checkout);
the issue quotes its overall assessment as rerunning "the complete
acceptance and PTY verification suite with deterministic handshakes".
Relevant PRD section: *Testing Decisions* in
[`Notes/PRD-vrg.md`](../PRD-vrg.md). This is the post-audit counterpart
of the Issue #35 pass recorded in
[final-verification.md](final-verification.md); the evidence record
lives in the
[walkthrough](../walkthroughs/050-01/code-walkthrough/walkthrough.md).

## `scripts/verify.sh` — the permanent gate sequence

The repository's single ordered verification entry point, runnable from
a clean checkout. The first non-zero gate stops the run and reports the
failing gate; a final line reports all gates green:

1. `go build ./...`
2. `go vet ./...`
3. `go build -tags vrg_testhooks ./cmd/vrg`
4. `go vet -tags vrg_testhooks ./cmd/vrg`
5. `go test ./... -count=1` — uncached
6. `CGO_ENABLED=1 go test -race ./... -count=1`
7. `go test ./cmd/vrg -count=3` — the real-binary PTY suite three times
8. `go mod verify`
9. `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` — pinned
   version; needs network access or a warm tool/module/vulnerability
   cache. A fetch/database failure is an environment prerequisite
   failure (exit 75), distinguished from a repository failure, not a
   gate defect.
10. `go mod tidy -diff` — the Issue #49 tidy manifest proof.

Gates 3–4 keep the `vrg_testhooks` variant under build and vet so the
tagged halves of the [hook boundaries](test-hook-topology.md) cannot
rot while the untagged gates exercise the released shape.

## The `FAKE_RG_*` fixture rename

Fixture-owned environment variables — read only by the fake-rg scripts
the tests install on `PATH`, never by vrg — were renamed off the
`VRG_TEST_` prefix so the prefix unambiguously means "seam the tagged
binary consumes":

| Old name | New name |
|---|---|
| `VRG_TEST_HANDSHAKE` | `FAKE_RG_HANDSHAKE_FILE` |
| `VRG_TEST_ARGV` | `FAKE_RG_ARGV_FILE` |
| `VRG_TEST_CWD` | `FAKE_RG_CWD_FILE` |
| `VRG_TEST_RG_PID` | `FAKE_RG_PID_FILE` |
| `VRG_TEST_RG_READY` | `FAKE_RG_READY_FILE` |

The rename touched every active PTY fixture (`internal/app/rg_test.go`
and the `cmd/vrg` test files) and the hook-manifest commentary; the
vrg-consumed [hook manifest](test-hook-topology.md) names
(`VRG_TEST_REAP`, `VRG_TEST_GATE`, `VRG_TEST_EVENT_ACK`, and the rest)
stay `VRG_TEST_*` because the tagged binary really does read them. The
untagged `go test -count=1 ./cmd/vrg` suite was run before and after
the rename.

## `scripts/smoke.py` — the canonical condition-driven harness

The canonical smoke harness, distinct from the frozen historical Issue
#35 record at
`Notes/walkthroughs/035-03/code-walkthrough/smoke.py` (unchanged — its
walkthrough transcript is a completed artifact). It drives the
**untagged** production binary (`go build -o /tmp/vrg-smoke/vrg
./cmd/vrg` — no test seams compiled in, and the smoke environment
asserts no `VRG_TEST_*` name is set) through a real PTY with explicit
conditions instead of fixed delays:

- every key send follows an observed UI/output marker;
- process, pipe, and PTY-EOF completion are drained on real events;
- a static assertion rejects `time.sleep` in the harness itself;
- cancellation additionally observes the child pid *and* its process
  group gone externally, PTY EOF, and terminal restoration (cursor
  visible, alt-screen left, termios equal to pre-launch).

A ready file alone is never treated as evidence of an application
state — that distinction belongs to the tagged-build acknowledgement
records of [pty-handshakes.md](pty-handshakes.md).

## Closing verification results

- `scripts/verify.sh`: all ten gates green.
- Explicit reruns: `go test -count=1 ./cmd/vrg` and `CGO_ENABLED=1 go
  test -race -count=1 ./cmd/vrg` — the Issue #48 acknowledgement
  handshake suite — both green.
- `TestNoFixedSleepsInPTYHelpers` green: no fixed settling/inter-key
  `time.Sleep` remains outside the allow-listed bounded condition polls
  (now including `waitProbeGone`).
- The five smoke outcomes against the untagged binary:
  browse → 0; no-results → 1; fatal → 2 under both `q` and `Esc` with
  the composed overlay diagnostics (exit-code process line, integrity
  causes, record-loss lines — see
  [error-overlay-and-outcomes.md](error-overlay-and-outcomes.md) and
  [integrity-diagnostics.md](integrity-diagnostics.md)) replayed to
  stderr; cancellation → 130 under both `q` and `ctrl+c` with child
  pid, process group, PTY EOF, and termios all observed restored/gone;
  help-only → 0 for bare/`-h`/`--help` with exactly one `Usage:` copy
  on stdout and the sentinel fake rg never invoked.

## Regression found and repaired

The first smoke run hung on cancellation: `Child.Terminate` killed only
the direct `rg` pid, and the fixture's non-`exec` `sleep` descendant
inherited the output pipes — collection drains pipes to EOF before
`cmd.Wait()`, so the surviving descendant deadlocked the exit. Repaired
against the owning contract — Issue #4's terminate-and-reap guarantee
([cancellation-cleanup.md](cancellation-cleanup.md)) — not by weakening
tests: `spawn` now makes the child a process-group leader and
`Terminate`/the context-cancel path kill the whole group
(`internal/app/rg.go` with the unix/other platform split in
`rg_unix.go`/`rg_other.go`). Coverage:
`internal/app/rg_unix_test.go`'s `TestTerminateKillsChildProcessGroup`
at the process boundary and `cmd/vrg/cancel_test.go`'s
`TestCancelTerminatesChildProcessGroup` end to end through the PTY
harness — see [unit-tests.md](unit-tests.md).
