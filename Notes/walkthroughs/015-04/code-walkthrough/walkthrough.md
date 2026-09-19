# Issue #15: file-change pop-up

*2026-09-17T14:01:16Z by Showboat 0.6.1*
<!-- showboat-id: b59b7a0c-e21e-4d22-80b5-b4074f6a5352 -->

Issue #15 adds the file-change pop-up: when n/p navigation lands on a stop in a different file, a bordered box carrying the destination's escaped path floats centred over the browse frame for one second or until the next key press — and that key still performs its normal action in the same update. Each crossing mints a fresh instance-keyed timer: a stale expiry can never dismiss a newer pop-up, and load completion never restarts it. Centring and left-truncation with a leading … are computed from the current terminal size at every render, so a resize recentres and re-truncates without touching the timer. An error overlay opening cancels the pop-up permanently. See Notes/issues/015-file-change-popup.md, Notes/tasks/015-file-change-popup.md, and the 'Colours, overlays, and key precedence' (pop-up bullet) and 'File loading' (selection-time start) sections of Notes/PRD-vrg.md — user stories 58 and 59. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Model tests — the instance-keyed lifecycle

internal/app/popup_test.go drives the whole contract through Update with the popupStubTicks options seam — an instantly resolving expiry command — while tests inject popupExpireMsg values with explicit instance IDs; no sleeps anywhere. Navigation's tea.Batch is unwrapped leaf by leaf (leafMsgs/navLeafMsgs) and only the fileLoadedMsg leaf is delivered back (deliverLoad). The suite covers: selection-time start over the still-loading panel with load completion leaving the live instance untouched; centring on both plain and styled themes; a stale instance's expiry discarded while the newer instance answers its own; any key press dismissing AND acting in the same update (down scrolls, q quits, Esc only dismisses); resize recentring and re-truncating the same live instance without restarting its timer; an error overlay cancelling the pop-up with no return after dismissal; an over-wide path left-truncated with a leading …; and a hostile path (newline, ESC, invalid UTF-8) rendered as the single-line escaped form.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'Popup' ./internal/app 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestPopupStartsAtSelectionNotLoad
--- PASS: TestPopupCentredOnFrame
--- PASS: TestPopupStaleExpiryCannotDismissNewer
--- PASS: TestPopupKeyDismissesAndActs
--- PASS: TestPopupEscDismissesOnly
--- PASS: TestPopupRecentresOnResizeWithoutRestart
--- PASS: TestPopupCancelledByErrorOverlay
--- PASS: TestPopupLeftTruncatesLongPath
--- PASS: TestPopupEscapesHostilePath
ok  	vrg/internal/app
```

## Sink safety — the pop-up joins the table

The Issue #6 sink-safety table gains a 'file-change pop-up' row: popupFixtureView drives every hostile fixture through the real composition path — an n across the boundary into a fixture-named file — under both the plain and dark themes, asserting the escaped path reached the raw output and no fixture payload follows an unescaped ESC.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestSinkSafetyTable/file-change' ./internal/app 2>&1 | grep -E '^(=== RUN|--- |    --- |ok|FAIL)' | grep -v '=== RUN' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//' | head -30
```

```output
--- PASS: TestSinkSafetyTable
    --- PASS: TestSinkSafetyTable/file-change_pop-up/osc_title-set
    --- PASS: TestSinkSafetyTable/file-change_pop-up/csi_erase
    --- PASS: TestSinkSafetyTable/file-change_pop-up/c0_run
    --- PASS: TestSinkSafetyTable/file-change_pop-up/c1_nel
    --- PASS: TestSinkSafetyTable/file-change_pop-up/del
    --- PASS: TestSinkSafetyTable/file-change_pop-up/standalone_cr
    --- PASS: TestSinkSafetyTable/file-change_pop-up/invalid_utf-8
    --- PASS: TestSinkSafetyTable/file-change_pop-up/embedded_filename_newline
ok  	vrg/internal/app
```

## Manual PTY — the pop-up on the real binary

pty_popup.py builds a fixture — a.txt and b.txt with one 'hit' match each, plus a file whose name carries hostile bytes (embedded newline, ESC, an invalid UTF-8 byte) — and runs the real binary on a 100x24 pty, replaying the byte stream through a small screen emulator. It proves: no pop-up at startup; n across the boundary shows the centred bordered box while the filename rule already names b.txt — the pop-up starts at selection, not load completion; a quick second n dismisses the first pop-up AND navigates in the same press — the fresh instance shows the new file's name with every hostile byte escaped (newline as backslash-n, ESC as ^[, the invalid byte as \xff) on a single row and the frame intact; the pop-up repaints away about a second later on its own; n wrapping back to a.txt opens another fresh instance; and shrinking the pty to 60x15 recentres and left-truncates the same live pop-up with a leading … — no key press involved.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/015-04/code-walkthrough/vrg ./cmd/vrg && cd Notes/walkthroughs/015-04/code-walkthrough && python3 pty_popup.py
```

```output
init      : no pop-up box
n->b      : box top=10 left=11 row='│ /home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough/fixture/./b.txt │'
n->evil   : box top=10 left=0 row='│ /home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough/fixture/./zz-evil\\nname^[[2J\\xff.txt │'
expire    : pop-up expired on its own — the one-second timer
n->a      : box top=10 left=11 row='│ /home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough/fixture/./a.txt │'
resize    : box top=6 left=0 row='│ …es/walkthroughs/015-04/code-walkthrough/fixture/./a.txt │'
resize    : recentred and truncated on 60x15
q         : exit 0
OK
```

## Full suite — go test ./... and the race detector

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//' && CGO_ENABLED=1 go test -count=1 -race ./internal/... ./cmd/vrg 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
?   	vrg/Notes/walkthroughs/007-04/code-walkthrough/fixture	[no test files]
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
?   	vrg/internal/safepresentation/sinktest	[no test files]
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
ok  	vrg/cmd/vrg
```
