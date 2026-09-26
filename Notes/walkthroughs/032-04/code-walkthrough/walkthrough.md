# Issue #32: Overlay precedence — error suspends help, append preserves scroll, Esc/q dismissal semantics

*2026-09-24T19:19:49Z by Showboat 0.6.1*
<!-- showboat-id: 21ebb87d-326f-4ae1-bf66-2ab24cc0b953 -->

Walkthrough for [Issue #32](../../../issues/032-overlay-precedence-esc-semantics.md), pinning the overlay-precedence and dismissal semantics per `Notes/PRD-vrg.md` (*Colours, overlays, and key precedence* — the precedence stack, the `Esc` bullet, the error/help bullets — and *Outcome and exit-status contract* — the outcome table and the `q`/`Esc` dismissal bullet): in browse/no-results states keys route `ctrl+c` → modal error → help → pop-up → base; a new error while help is open suspends it at its retained scroll position and either dismissal key restores it; appended errors keep the reader's scroll position; opening help or an error cancels any pop-up with no return; and `Esc` is an overlay-dismissal key only — a no-op in every base state, while dismissing the fatal no-results overlay with either key exits 2 because there is no underlying state. The composed semantics were already assembled from the Issue #9/#15/#26/#31 primitives (`openOverlay`'s open-or-append and pop-up cancellation, the overlay-first key routing, `overlayKey`'s fatal-quit dismissal, `Esc` unbound in base states), so this issue's code deliverable is `internal/app/precedence_test.go` — the contract tests that pin the stack — plus this verification. All generated artifacts live in this directory: the built `vrg` binary, the `demo-precedence.sh` tmux harness, its generated fake-rg scripts and `run/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/ cmd/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/032-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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

## Model tests — `internal/app/precedence_test.go`

`TestErrorSuspendsHelpRestoringScroll` runs both dismissal keys through the gated route the issue specifies: the load gate parks the current file's failing worker, `?` opens help at a reduced 80×10 height, five `down` presses scroll it to row 5, and releasing the gate lands the failure while help is open — the error overlay takes the keyboard and the topmost render while `m.help` holds its position; `down` then scrolls the error (not the help), `h`/`n`/`r`/`w`/`c` are all swallowed, and `q` or `Esc` dismisses the error to reveal help still at row 5. This is the error-while-help-open case the manual route cannot produce — the gate is what makes it deterministic. `TestAppendedErrorPreservesReaderScroll` generalizes Issue #26's append-preserving-scroll primitive past reload re-entry: a read-failure overlay scrolled to row 3 receives an unsupported-encoding append from a different error source, and the reader stays at 3 with the new text reachable at the bottom. `TestDismissalOutcomeTable` runs every dismissal-outcome row for both `q` and `Esc`: browse error → browsing, browse help → browsing, error-over-help → help restored at scroll 4 then a second dismissal → browsing then a base-state `q` exits, warning-over-empty → no-results, help-over-no-results → no-results, fatal and record-loss with no usable results → exit 2 — with the state-specific follow-ups: a second `q` exits the fixed status from each still-running base state while a second `Esc` is a no-op. The adjacent no-op and pop-up-cancellation tests are shown alongside for completeness.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestErrorSuspendsHelpRestoringScroll|TestAppendedErrorPreservesReaderScroll|TestDismissalOutcomeTable|TestEscOnBrowseIsNoOp|TestEscOnNoResultsIsNoOp|TestEscDuringSearchingIsNoOp|TestPopupDismissalKeys|TestHelpCancelsPopup|TestErrorOverlayCancelsPopup" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestEscOnBrowseIsNoOp
--- PASS: TestHelpCancelsPopup
--- PASS: TestEscDuringSearchingIsNoOp
--- PASS: TestEscOnNoResultsIsNoOp
--- PASS: TestPopupDismissalKeys
--- PASS: TestErrorOverlayCancelsPopup
--- PASS: TestErrorSuspendsHelpRestoringScroll
    --- PASS: TestErrorSuspendsHelpRestoringScroll/q
    --- PASS: TestErrorSuspendsHelpRestoringScroll/esc
--- PASS: TestAppendedErrorPreservesReaderScroll
--- PASS: TestDismissalOutcomeTable
    --- PASS: TestDismissalOutcomeTable/browse_error_overlay_returns_to_browsing/q
    --- PASS: TestDismissalOutcomeTable/browse_help_returns_to_browsing/q
    --- PASS: TestDismissalOutcomeTable/browse_error-over-help_restores_help_then_browse_then_exits/q
    --- PASS: TestDismissalOutcomeTable/warning_overlay_over_empty_result_returns_to_no-results/q
    --- PASS: TestDismissalOutcomeTable/help_over_no-results_returns_to_no-results/q
    --- PASS: TestDismissalOutcomeTable/fatal_overlay_with_no_usable_results_exits_2/q
    --- PASS: TestDismissalOutcomeTable/record-loss_overlay_with_no_usable_results_exits_2/q
    --- PASS: TestDismissalOutcomeTable/browse_error_overlay_returns_to_browsing/esc
    --- PASS: TestDismissalOutcomeTable/browse_help_returns_to_browsing/esc
    --- PASS: TestDismissalOutcomeTable/browse_error-over-help_restores_help_then_browse_then_exits/esc
    --- PASS: TestDismissalOutcomeTable/warning_overlay_over_empty_result_returns_to_no-results/esc
    --- PASS: TestDismissalOutcomeTable/help_over_no-results_returns_to_no-results/esc
    --- PASS: TestDismissalOutcomeTable/fatal_overlay_with_no_usable_results_exits_2/esc
    --- PASS: TestDismissalOutcomeTable/record-loss_overlay_with_no_usable_results_exits_2/esc
PASS
ok  	vrg/internal/app
```

## Manual route — the real binary on a PTY

`demo-precedence.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with two generated fake-`rg` scripts on `PATH` — repository and user files are never touched; the fixture lives in a disposable `mktemp` directory an EXIT trap removes.

Session 1's fake rg emits a complete two-file stream plus a stderr warning and exits 0: the warning overlay opens over the browse screen at startup; `Esc` dismisses it to reveal a.txt; a second `Esc` with no overlay is a byte-identical no-op — the frame compares equal and the program keeps running; `?` opens help and `down`×3 scrolls it, `Esc` closes it, and a reopened `r` under help is ignored — no `Loading…` flash, help stays up. Then `chmod 000 b.txt` and `n` cross into the unreadable file: the file-change pop-up flashes at selection and the arriving error overlay (`cannot read b.txt`) cancels it; `q` closes the overlay only — browsing is still running under the `(unreadable)` placeholder — and the second `q` exits with the fixed status 0, the stderr replay carrying both the warning and the failure.

Sessions 2 and 3 run the fatal route: a fake rg that exits 3 with no output produces the fatal overlay standing alone — `exit status 3` with no browse beneath — and dismissal terminates with status 2 under `Esc` and `q` alike, the diagnostic replayed to stderr after terminal restoration. The gated error-while-help-open restoration case is covered by the deterministic model test above — the issue notes the manual route cannot produce an error while help is open.

```bash
cd /home/chris/vrg/Notes/walkthroughs/032-04/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-precedence.sh
```

```output
ok: startup: warning overlay over browse
ok: Esc: warning overlay dismissed
ok: Esc: browse revealed
ok: Esc no overlay: frame unchanged -> same
ok: Esc no overlay: still running
ok: ?: help opens over browse
ok: Esc: help closed
ok: Esc: browse intact
ok: r under help: help still open
ok: r under help: no reload started
ok: n into b.txt: error overlay opens
ok: n into b.txt: overlay names the path
ok: n into b.txt: (unreadable) placeholder
ok: q: error overlay closed
ok: q: still browsing
ok: q: program still running
ok: vrg exit status -> 0
ok: stderr replays the warning -> 1
ok: stderr replays the read failure -> 1
ok: fatal rg 3: overlay stands alone
ok: fatal rg 3: no browse beneath
ok: vrg exit status -> 2
ok: fatal rg 3 Escape: stderr replays the diagnostic -> 1
ok: fatal rg 3: overlay stands alone
ok: fatal rg 3: no browse beneath
ok: vrg exit status -> 2
ok: fatal rg 3 q: stderr replays the diagnostic -> 1
demo-precedence: all checks passed
```

The deterministic model tests remain the authority for the precedence stack, the suspension and append rules, the pop-up cancellation, and the dismissal-outcome table; the PTY route demonstrates the visible behavior end to end — the warning overlay opening over browse, the no-overlay `Esc` no-op, the ignored `r` under help, the pop-up cancelled by the arriving error, the `q` dismissal that leaves browsing running before the base-state `q` exits 0, and the fatal overlay exiting 2 under both dismissal keys. Wiki ingest lives in `Notes/wiki/overlay-precedence.md`.
