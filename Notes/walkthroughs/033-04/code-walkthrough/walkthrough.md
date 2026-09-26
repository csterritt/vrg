# Issue #33: Terminal too small — gated screen with full state recovery

*2026-09-24T19:49:21Z by Showboat 0.6.1*
<!-- showboat-id: bc470279-ffb9-4d20-be44-ae0a55bfac29 -->

Walkthrough for [Issue #33](../../../issues/033-terminal-too-small-with-state-recovery.md), implementing the too-small gate per `Notes/PRD-vrg.md` (*Layout and indicators* — the minimum-size bullet — and *Outcome and exit-status contract*): below 20 columns or 3 rows the whole screen is a centred "Terminal too small" message, only `q` and `ctrl+c` act, and every other key is a no-op that leaves the hidden state untouched — cursor, per-file viewport anchors, list visibility, wrap, colour, horizontal offset, the full modal stack with its scroll positions, and the live pop-up timer all freeze behind the gate and return intact on recovery. `q` exits with the state-applicable status rather than dismissing a logically open overlay, taking precedence over Issue #32's dismissal semantics. The implementation lives in `internal/app/toosmall.go` (the `tooSmall` predicate, the `tooSmallKey` keyboard, and `renderTooSmall`) plus the gate calls in `Update`, `View`, and `syncLayout`. All generated artifacts live in this directory: the built `vrg` binary, the `demo-toosmall.sh` tmux harness, and its generated fake-rg script and `run/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/ cmd/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/033-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
```

```output
GOFMT-CLEAN
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/internal/app
GATES-OK
```

## Model tests — `internal/app/toosmall_test.go`

The contract tests drive `Update` alone. `TestTooSmallThreshold` pins the 20×3 boundary in both directions — 19×10 and 40×2 are gated, exactly 20×3 is viable. `TestTooSmallMessageCentredAsSpacePermits` checks the centred message at 19×10 and 40×2 and its clipping at 10×1. `TestTooSmallQExitByState` runs the state-applicable `q` table — fixed status while browsing, 1 on no-results, 2 with the fatal overlay logically open, and a plain exit past a browse error overlay, help, or an error-over-help stack rather than a dismissal — with `TestTooSmallQWhileSearchingCancels` covering the 130 cancellation path and `TestTooSmallCtrlCExits130` the unconditional one. `TestTooSmallKeysAreNoOps` proves `Esc`, `n`, `w`, and friends produce no command and no state change under an error-over-help stack that reopens at its scroll positions after recovery. The round-trip pair — `TestTooSmallRoundTripRestoresBrowseState` and `TestTooSmallRoundTripRestoresModalState` — shrink and grow past the gate and compare cursor, saved per-file anchors, the current anchor and viewport top, horizontal offset, list visibility, wrap, colour, and the scrolled help / scrolled error / error-over-help modal stack at their prior positions. `TestTooSmallInteriorResizeDefersRecovery` drives the 19×2 → 10×1 → 25×8 chain: no layout request minted at 10×1, no anchor or modal movement, recovery keyed to the final dimensions. `TestTooSmallCompletionBehindGate` resolves a search completing under the gate. The pop-up pair — `TestTooSmallPopupTimerContinues` and `TestTooSmallPopupHiddenNotCancelled` — proves the timer is never restarted, inert keys under the gate cannot dismiss the hidden instance, an expiry during too-small dismisses it for good, and a live instance reappears after recovery.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestTooSmall" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestTooSmallThreshold
    --- PASS: TestTooSmallThreshold/19x24
    --- PASS: TestTooSmallThreshold/19x3
    --- PASS: TestTooSmallThreshold/40x2
    --- PASS: TestTooSmallThreshold/20x2
    --- PASS: TestTooSmallThreshold/10x1
    --- PASS: TestTooSmallThreshold/20x3
    --- PASS: TestTooSmallThreshold/21x4
    --- PASS: TestTooSmallThreshold/80x24
--- PASS: TestTooSmallMessageCentredAsSpacePermits
--- PASS: TestTooSmallQExitByState
    --- PASS: TestTooSmallQExitByState/browsing_exits_the_fixed_status_0
    --- PASS: TestTooSmallQExitByState/browsing_under_a_fatal_search_exits_the_fixed_status_2
    --- PASS: TestTooSmallQExitByState/no-results_exits_1
    --- PASS: TestTooSmallQExitByState/fatal_no-results_overlay_logically_open_exits_2
    --- PASS: TestTooSmallQExitByState/browse_error_overlay_logically_open_exits_rather_than_dismissing
    --- PASS: TestTooSmallQExitByState/help_logically_open_exits_rather_than_closing
    --- PASS: TestTooSmallQExitByState/error-over-help_stack_exits_rather_than_dismissing
--- PASS: TestTooSmallQWhileSearchingCancels
--- PASS: TestTooSmallCtrlCExits130
    --- PASS: TestTooSmallCtrlCExits130/searching
    --- PASS: TestTooSmallCtrlCExits130/browsing
    --- PASS: TestTooSmallCtrlCExits130/no-results
    --- PASS: TestTooSmallCtrlCExits130/fatal_overlay_logically_open
--- PASS: TestTooSmallKeysAreNoOps
--- PASS: TestTooSmallRoundTripRestoresBrowseState
--- PASS: TestTooSmallRoundTripRestoresModalState
    --- PASS: TestTooSmallRoundTripRestoresModalState/scrolled_help
    --- PASS: TestTooSmallRoundTripRestoresModalState/scrolled_error_overlay
    --- PASS: TestTooSmallRoundTripRestoresModalState/error_over_help_stack
--- PASS: TestTooSmallInteriorResizeDefersRecovery
    --- PASS: TestTooSmallInteriorResizeDefersRecovery/scrolled_help_and_viewport
    --- PASS: TestTooSmallInteriorResizeDefersRecovery/error_over_help_and_viewport
--- PASS: TestTooSmallCompletionBehindGate
--- PASS: TestTooSmallPopupTimerContinues
--- PASS: TestTooSmallPopupHiddenNotCancelled
PASS
ok  	vrg/internal/app
```

## Manual route — the real binary on a PTY

`demo-toosmall.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 40×10 with a generated fake-`rg` script on `PATH` — repository and user files are never touched; the fixture lives in a disposable `mktemp` directory an EXIT trap removes. The fake rg emits a complete two-file stream and exits 0, so browse opens on a.txt with no overlay in the way.

Session 1 is the required walkthrough: `?` opens help over browse, four `Down` presses scroll binding rows off the top (the frame provably changes and the first bindings leave the window). `tmux resize-window` then drops the PTY to 15×2: the gate replaces the whole frame — the message clipped to "Terminal too sm" at that width — with the help box hidden but logically open, and an `Esc` there is a byte-identical no-op that leaves the program running. Growing to exactly 20×3 lifts the gate (the boundary is viable) and the help box renders clipped to the tiny frame; growing back to 40×10 restores a frame byte-identical to the pre-shrink capture — the scroll position survived the round trip untouched. A second shrink to 15×2 reinstates the gate, and `q` there exits the program outright with the fixed browse status 0 — it does not merely dismiss the logically open help. Session 2 repeats the shrink and sends `ctrl+c` for the unconditional exit 130.

```bash
cd /home/chris/vrg/Notes/walkthroughs/033-04/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-toosmall.sh
```

```output
ok: ?: help opens over browse
ok: Down x4: help scrolled -> different
ok: Down x4: top bindings scrolled off
ok: 15x2: gate message (clipped to width)
ok: 15x2: help hidden behind the gate
ok: Esc at 15x2: frame unchanged -> same
ok: Esc at 15x2: still running
ok: 20x3: boundary viable — gate lifted
ok: 20x3: help box clipped to the frame
ok: 20x3: still running
ok: 40x10 recovery: help frame identical -> same
ok: vrg exit status -> 0
ok: vrg exit status -> 130
demo-toosmall: all checks passed
```

The deterministic model tests remain the authority for the threshold, the state-applicable exit table, the inert-key contract, the interior-resize deferral, and the pop-up timer cases; the PTY route demonstrates the visible behavior end to end — the centred gate message, the hidden help surviving a 15×2 → 20×3 → 40×10 round trip at its exact scroll offset, the `Esc` no-op under the gate, and `q` exiting past the logically open modal rather than dismissing it. Wiki ingest lives in `Notes/wiki/terminal-too-small.md`.
