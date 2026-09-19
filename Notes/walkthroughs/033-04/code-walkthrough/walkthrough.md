# Issue #33: "Terminal too small" gate with full state recovery

*2026-09-18T02:43:02Z by Showboat 0.6.1*
<!-- showboat-id: eae31f46-c3eb-4239-836a-0d0c7a876b3f -->

Issue #33 installs the too-small gate: below the fixed 20-column/3-row minimum the whole ordinary presentation and key map are replaced by the centred, clipped 'Terminal too small' note — only q and ctrl+c act — and growing back restores the session exactly. The implementation is boundary logic, not state transformation: the sized field separates 'no size reported' from a reported sub-minimum size; WindowSizeMsg records dimensions then skips all layout and modal work while gated; layoutReadyMsg discards completions arriving sub-minimum; requestLayout issues nothing; KeyPressMsg routes to tooSmallKey (q exits the state-applicable status — 130 searching, the fixed status browsing, 1 on no-results, 2 with the fatal overlay logically open — exiting past a logically open overlay rather than demoting to an Issue #32 dismissal, while Esc and every other key are strict no-ops); and View short-circuits to the note with no state, modal, or pop-up compositing. Pop-up timers keep running behind the gate — an expiry landing there dismisses permanently, a live instance reappears on growth — and resizes wholly inside the gate mutate nothing, recovery running at the final dimensions. internal/app/toosmall_test.go pins the contracts end to end. See Notes/issues/033-terminal-too-small-with-state-recovery.md, Notes/tasks/033-terminal-too-small-with-state-recovery.md, and the 'Layout and indicators' minimum-size bullet of Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Model tests — the threshold and the display

TestTooSmallThresholdAndDisplay drives the boundary itself through Update/View: 19x10 installs the gate (the note alone, centred on row 4 — nothing composites over it, not even 'Searching…'), 40x2 gates on height alone with the note centred horizontally, a 10-column frame clips the note to 'Terminal t' rather than overflowing, and exactly 20x3 is the ordinary boundary where the searching screen renders.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestTooSmallThresholdAndDisplay' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestTooSmallThresholdAndDisplay
ok  	vrg/internal/app
```

## Model tests — exit semantics and the no-op key map

TestTooSmallQExitsPerState runs the per-state q table: 130 while searching, the fixed search-derived status while browsing (before and after an ordinary dismissal), exit-2-past-the-overlay with a browse error logically open (the overlay stays logically open — q exits, it never demotes to an Issue #32 dismissal), 1 on no-results with or without a warning overlay logically open, and 2 with the fatal overlay logically open. TestTooSmallCtrlCExits130 pins the global override. TestTooSmallKeysAreNoOps sweeps 28 keys — Esc, navigation, scroll, pan, toggles, modal keys — and proves nothing behind the gate changes: cursor, loads, pop-up, theme, wrap, list visibility, and overlay scroll all hold, the overlay never renders, and growth reopens it at its scroll.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestTooSmallQExitsPerState|TestTooSmallCtrlCExits130|TestTooSmallKeysAreNoOps' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestTooSmallQExitsPerState
    --- PASS: TestTooSmallQExitsPerState/searching_exits_130
    --- PASS: TestTooSmallQExitsPerState/browse_exits_the_fixed_status
    --- PASS: TestTooSmallQExitsPerState/browse_error_overlay_exits_not_dismisses
    --- PASS: TestTooSmallQExitsPerState/browse_after_dismissal_exits_the_fixed_status
    --- PASS: TestTooSmallQExitsPerState/no-results_exits_1
    --- PASS: TestTooSmallQExitsPerState/no-results_warning_overlay_exits_1
    --- PASS: TestTooSmallQExitsPerState/fatal_overlay_exits_2
--- PASS: TestTooSmallCtrlCExits130
--- PASS: TestTooSmallKeysAreNoOps
ok  	vrg/internal/app
```

## Model tests — recovery, in-gate resizes, and the pop-up

TestTooSmallRoundTripPreservesModalStack reopens the modal stack at its positions: scrolled help at 5, a scrolled error at 7, and an error-at-3-over-suspended-help stack. TestTooSmallRoundTripPreservesBrowseState loses nothing of the browse state — cursor selection, both files' saved viewports (top, mid-line anchor, pan offset), wrap off, light theme, hidden list. TestTooSmallResizeWithinGateDefersRecovery runs 19x2 -> 10x1 -> 25x8: no layout command issued, no layout installed, no anchor or modal mutation inside the gate, and recovery runs at the final 25x8. TestTooSmallPopupContinuesWithoutDisplay proves the timer runs on behind the gate — expiry during too-small dismisses permanently, a live instance reappears — and the box never renders there.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestTooSmallRoundTripPreservesModalStack|TestTooSmallRoundTripPreservesBrowseState|TestTooSmallResizeWithinGateDefersRecovery|TestTooSmallPopupContinuesWithoutDisplay' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestTooSmallRoundTripPreservesModalStack
    --- PASS: TestTooSmallRoundTripPreservesModalStack/scrolled_help
    --- PASS: TestTooSmallRoundTripPreservesModalStack/scrolled_error
    --- PASS: TestTooSmallRoundTripPreservesModalStack/error_over_help
--- PASS: TestTooSmallRoundTripPreservesBrowseState
--- PASS: TestTooSmallResizeWithinGateDefersRecovery
--- PASS: TestTooSmallPopupContinuesWithoutDisplay
    --- PASS: TestTooSmallPopupContinuesWithoutDisplay/expiry_during_too-small_dismisses
    --- PASS: TestTooSmallPopupContinuesWithoutDisplay/live_pop-up_returns_on_recovery
ok  	vrg/internal/app
```

## Full module regression

The gate touches the shared Update/View path every screen consumes, so the whole suite runs, plus the race detector over internal/app.

```bash
cd /home/chris/vrg && go test -count=1 ./internal/... ./cmd/vrg 2>&1 | sed -E 's/\t[0-9.]+s$//' && CGO_ENABLED=1 go test -race -count=1 ./internal/app 2>&1 | sed -E 's/\t[0-9.]+s$//'
```

```output
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
```

## Manual check — the real binary on a real pty

manual_route.sh runs the issue's manual route against a disposable mktemp fixture — never a repository file: aaa-one.txt and bbb-two.txt with 'needle' matches, under a trap removing the tree on exit or interruption. pty_toosmall.py drives one session on a real 60x14 pty where the resizes are real TIOCSWINSZ calls — the kernel delivers SIGWINCH to the foreground process group, exactly what Bubble Tea listens to: '?' opens help, down x4 scrolls the table to 'ctrl+c' with the title off; the pty shrinks to 15x2 — under both minimums — and the frame becomes the clipped 'Terminal too sm' note alone; Esc there is a strict no-op; growing back to 60x14 restores help at the exact scrolled position; a second shrink and q exits the fixed search status 0.

```bash
cd /home/chris/vrg && bash Notes/walkthroughs/033-04/code-walkthrough/manual_route.sh
```

```output
fixture: /tmp/tmp.tMFRrTR6Cb (aaa-one.txt, bbb-two.txt — 'needle' matches)
? down x4   : help open and scrolled — 'ctrl+c' in, title out
resize      : 15x2 — clipped 'Terminal too sm', nothing behind
Esc         : inside the gate — strict no-op, still running
resize      : 60x14 — help restored at its scrolled position
q           : exit 0 — q inside the gate exits the fixed status
OK
```

Artifacts in this directory: vrg (the built binary the pty session ran), manual_route.sh (the disposable-fixture driver), pty_toosmall.py (the pty emulator and assertions — including DECSTBM-region scrolling, IND, ECH handling, and the TIOCSWINSZ resize that drives SIGWINCH).
