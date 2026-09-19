# Issue #32: overlay precedence — suspension, appends, and the q/Esc dismissal table

*2026-09-18T02:00:23Z by Showboat 0.6.1*
<!-- showboat-id: 20152a6c-5e60-4125-9574-4148df98b47b -->

Issue #32 formalizes and verifies the overlay-precedence stack and dismissal semantics the earlier overlay issues composed: ctrl+c over the modal error overlay over modal help over the file-change pop-up over base keys. internal/app/precedence_test.go pins the contracts end to end — an error arriving while help is open suspends it and either dismissal key (q or Esc) restores help at its retained scroll position; a second error appended to a scrolled overlay keeps the reader's position with the new text reachable, generalizing Issue #26's append-preserving-scroll primitive to all appended errors; opening help or an error cancels a live pop-up permanently; Esc never exits a base state and is a strict no-op in bare browsing and on the no-results screen; and the full dismissal-outcome table runs for both q and Esc — browse and no-results overlays dismiss to running base states whose follow-up q exits the fixed status or 1, while the fatal no-results and record-loss overlays exit 2 on the first dismissal because there is no underlying state, making that Esc exit the single exception to Esc-never-exits. No production change was needed: Issues #15, #26, and #31 already deliver the stack — these tests prove the combined behavior. See Notes/issues/032-overlay-precedence-esc-semantics.md, Notes/tasks/032-overlay-precedence-esc-semantics.md, and the 'Colours, overlays, and key precedence' and 'Outcome and exit-status contract' sections of Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Model tests — suspension and error-first routing

newSuspendRig builds the error-over-help fixture: the model browses a.txt, n crosses into b.txt minting a load request the test holds unrun, help opens and scrolls to position 5, and the held load is released as a failure while help is open. TestErrorSuspendsHelpRestoringScroll runs both dismissal keys — the error opens at scroll 0 with help retained at 5 underneath, and q or Esc closes only the error, restoring help at row 5 with the title scrolled off. TestErrorOverHelpRoutesKeysToError proves the error case wins while the stack is up — up/down scroll the error, every other key (h, ?, r, the base bindings) is ignored with the state behind untouched, and a second error appended keeps the reader at 2 with help still suspended. TestCtrlCOverErrorOverHelpExits130 tops the stack.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestErrorSuspendsHelpRestoringScroll|TestErrorOverHelpRoutesKeysToError|TestCtrlCOverErrorOverHelpExits130' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestErrorSuspendsHelpRestoringScroll
    --- PASS: TestErrorSuspendsHelpRestoringScroll/esc
    --- PASS: TestErrorSuspendsHelpRestoringScroll/q
--- PASS: TestErrorOverHelpRoutesKeysToError
--- PASS: TestCtrlCOverErrorOverHelpExits130
ok  	vrg/internal/app
```

## Model tests — append-preserving scroll

TestAppendedErrorPreservesReaderPosition generalizes Issue #26's primitive: a fatal search (rg exit 3 plus 30 stderr diagnostics) opens the scrollable error overlay, the reader scrolls to 3, and the current file's load failure appends to the open overlay — the reader stays at 3, both texts are in the model, and scrolling to the bottom reaches the appended failure without disturbing the browse state.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestAppendedErrorPreservesReaderPosition' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestAppendedErrorPreservesReaderPosition
ok  	vrg/internal/app
```

## Model tests — the dismissal-outcome table, Esc no-ops, pop-up cancellation

TestDismissalOutcomeTable runs the whole table for both q and Esc: a browse error overlay dismisses to browsing still running (a base q then exits the fixed 2); browse help closes to browsing (base q exits 0); error-over-help takes the full three-key sequence — close error, help restored at scroll 5, close help to browsing, then base q exits 0; the empty-result warning and no-results help close to the no-results screen (base q exits 1); and the fatal no-results and record-loss overlays exit 2 on the first dismissal — Esc's only terminating dismissal, because there is no underlying state. Every still-running row takes the state-specific follow-ups: a second Esc leaves it running, a second q exits the fixed status — never a uniform second-q-exits assertion. TestEscWithNoOverlayIsNoOp pins Esc as a strict no-op in bare browse and on bare no-results. TestErrorArrivalCancelsPopup and the earlier TestHelpCancelsPopup pin both cancellation routes — no suspended pop-up returns.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestDismissalOutcomeTable|TestEscWithNoOverlayIsNoOp|TestErrorArrivalCancelsPopup|TestHelpCancelsPopup' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestHelpCancelsPopup
--- PASS: TestErrorArrivalCancelsPopup
--- PASS: TestEscWithNoOverlayIsNoOp
--- PASS: TestDismissalOutcomeTable
    --- PASS: TestDismissalOutcomeTable/q:_browse_error_overlay_to_browsing
    --- PASS: TestDismissalOutcomeTable/q:_browse_help_to_browsing
    --- PASS: TestDismissalOutcomeTable/q:_browse_error-over-help_restores_help
    --- PASS: TestDismissalOutcomeTable/q:_empty_result_warning_to_no-results
    --- PASS: TestDismissalOutcomeTable/q:_no-results_help_to_no-results
    --- PASS: TestDismissalOutcomeTable/q:_fatal_no_usable_results_exits_2
    --- PASS: TestDismissalOutcomeTable/q:_record-loss_no_results_exits_2
    --- PASS: TestDismissalOutcomeTable/esc:_browse_error_overlay_to_browsing
    --- PASS: TestDismissalOutcomeTable/esc:_browse_help_to_browsing
    --- PASS: TestDismissalOutcomeTable/esc:_browse_error-over-help_restores_help
    --- PASS: TestDismissalOutcomeTable/esc:_empty_result_warning_to_no-results
    --- PASS: TestDismissalOutcomeTable/esc:_no-results_help_to_no-results
    --- PASS: TestDismissalOutcomeTable/esc:_fatal_no_usable_results_exits_2
    --- PASS: TestDismissalOutcomeTable/esc:_record-loss_no_results_exits_2
ok  	vrg/internal/app
```

## Full module regression

The new tests exercise the shared key routing every overlay consumes, so the whole suite runs, plus the race detector over internal/app.

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

manual_route.sh runs the issue's manual cases against a disposable mktemp fixture — never a repository file: aaa-readable.txt and bbb-blocked.txt with 'needle' matches, a bin/rg fake exiting 3 with no output, and a gate file for VRG_TEST_LOAD_GATE, under a trap removing the tree on exit or interruption. pty_precedence.py drives four sessions on a real 60x14 pty. Session one (real rg): Esc with no overlay open changes nothing; chmod 000 bbb then n into it opens the browse error overlay; the first q closes only the overlay — browse keeps running behind — and the second q exits the fixed status 0. Sessions two and three (fake rg exit 3, no output): the fatal overlay names 'ripgrep exited with code 3' and Esc and q each exit 2 — there is no underlying state, Esc's only terminating dismissal. Session four (real rg, VRG_TEST_LOAD_GATE set): the gate goes up, n into bbb mints a load held at the gate behind 'Loading…', '?' opens help, down x4 scrolls the table to 'ctrl+c' with the title scrolled off; releasing the gate lands the read failure over the suspended help; Esc closes only the error — help returns at its scrolled position; the second Esc closes help to '(unreadable)'; q exits 0.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/032-04/code-walkthrough/vrg ./cmd/vrg && bash Notes/walkthroughs/032-04/code-walkthrough/manual_route.sh
```

```output
fixture: <tmp> (aaa-readable.txt, bbb-blocked.txt — 'needle' matches; bin/rg exits 3)
Esc         : no overlay — frame untouched, still running
n           : browse error overlay 'cannot read …' is up
q           : overlay closed — browse running behind
q           : exit 0 — the fixed search status
rg -3       : fatal overlay — 'ripgrep exited with code 3'
Esc         : exit 2 — no underlying state to return to
rg -3       : fatal overlay — 'ripgrep exited with code 3'
q           : exit 2 — no underlying state to return to
n           : into bbb — 'Loading…', one load held at the gate
? down x4   : help open and scrolled — 'ctrl+c' in, title out
release     : error landed over help — 'cannot read …' on top
Esc         : error closed — help restored at scroll
Esc         : help closed — '(unreadable)' browse behind
q           : exit 0 — the fixed search status
OK
```

Artifacts in this directory: vrg (the built binary the pty sessions ran), manual_route.sh (the disposable-fixture driver), pty_precedence.py (the four-session pty emulator and assertions — including DECSTBM-region scrolling, IND, and ECH handling, which is how vrg's differential renderer repaints scrolled overlays).
