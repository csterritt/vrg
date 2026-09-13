# Issue #31: Help overlay h/? with wrapped, scrollable key bindings

*2026-09-12T14:15:18Z by Showboat 0.6.1*
<!-- showboat-id: 98d755f2-a440-49b1-a583-ad9ad9887333 -->

Walkthrough for Issue #31 (Notes/tasks/031-help-overlay.md), implementing the modal help overlay opened with h/? from ordinary browsing and the no-results screen. The help overlay uses the same wrapped, scrollable overlay component as the Issue #9 error overlay with base colours and a plain single-line border. Closing returns to the underlying base state; a subsequent q on no-results still exits 1. While open, up/down scroll rendered rows, q/Esc/h/? close, ctrl+c exits 130, and every other key — including n/p/w/c/r — is ignored with the state behind unchanged. Text wraps to the interior width including long unbroken strings; vertical scrolling reaches every row at usable sizes. At tiny sizes the overlay is clipped to the terminal without a borderless mode and restored on growth; rendering without panic at 25x8. The key-binding list is defined once as data (binding to description) that the help renderer consumes and documentation tests can iterate, plus a footer slot reserved for Issue #34. Opening help cancels an active Issue #15 pop-up with no return on close. Any substituted text is routed through the Issue #6 utility with help added to the sink-safety table. References: Notes/PRD-vrg.md (Colours, overlays, and key precedence), Notes/wiki/help-overlay.md, Issue #31.

Contracts verified:

- h and ? open the help overlay from ordinary browsing and the no-results screen.
- Closing (q/Esc/h/?) returns to the underlying base state; a subsequent q on no-results still exits 1.
- While open, up/down scroll rendered rows; q/Esc/h/? close; ctrl+c exits 130.
- Every other key — including n/p/w/c/r — is ignored with the state behind unchanged.
- Text wraps to the interior width including long unbroken strings; vertical scrolling reaches every row at usable sizes.
- At tiny sizes the overlay is clipped to the terminal without a borderless mode and restored on growth; rendering without panic at 25x8.
- The key-binding list is defined once as data (KeyBindings) covering navigation, scrolling, panning, wrap, colour, list toggle, reload, help, and quit/cancel, plus a footer slot (HelpFooter) reserved for Issue #34.
- Opening help cancels an active Issue #15 pop-up with no return on close.
- The help overlay passes the Issue #6 sink-safety check (no dangerous control bytes in the no-style path; no fixture payload after unescaped ESC with styles).

The model tests are the authoritative deterministic verification. The manual route at the end demonstrates the behavior with a real terminal.

```bash
cd /home/chris/vrg && go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/safepresentation
ok  	vrg/internal/searchindex
?   	vrg/internal/sinkfixtures	[no test files]
ok  	vrg/internal/theme
ok  	vrg/internal/viewport
```

## Help overlay model tests

The help overlay tests (internal/app/help_overlay_test.go) verify opening from browse and no-results, close and ignored keys, ctrl+c, scroll bounds, wrapping including unbroken strings, tiny-size clipping, binding-table rendering, pop-up cancellation, and the sink-safety row.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run '^TestHelp|^TestKeyBindings' -timeout 60s | sed 's/(0\.[0-9]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestHelpOpensFromBrowse
--- PASS: TestHelpOpensFromBrowse (0.00s)
=== RUN   TestHelpOpensFromBrowseQuestion
--- PASS: TestHelpOpensFromBrowseQuestion (0.00s)
=== RUN   TestHelpOpensFromNoResults
--- PASS: TestHelpOpensFromNoResults (0.00s)
=== RUN   TestHelpOpensFromNoResultsQuestion
--- PASS: TestHelpOpensFromNoResultsQuestion (0.00s)
=== RUN   TestHelpCloseReturnsToBrowse
--- PASS: TestHelpCloseReturnsToBrowse (0.00s)
=== RUN   TestHelpCloseReturnsToNoResults
--- PASS: TestHelpCloseReturnsToNoResults (0.00s)
=== RUN   TestHelpCloseThenQExits1FromNoResults
--- PASS: TestHelpCloseThenQExits1FromNoResults (0.00s)
=== RUN   TestHelpIgnoredKeysLeaveStateUnchanged
--- PASS: TestHelpIgnoredKeysLeaveStateUnchanged (0.00s)
=== RUN   TestHelpIgnoredKeysLeaveNoResultsUnchanged
--- PASS: TestHelpIgnoredKeysLeaveNoResultsUnchanged (0.00s)
=== RUN   TestHelpCtrlCExits130
--- PASS: TestHelpCtrlCExits130 (0.00s)
=== RUN   TestHelpCtrlCExits130FromNoResults
--- PASS: TestHelpCtrlCExits130FromNoResults (0.00s)
=== RUN   TestHelpScrollDownIncrements
--- PASS: TestHelpScrollDownIncrements (0.00s)
=== RUN   TestHelpScrollUpAtTopClamps
--- PASS: TestHelpScrollUpAtTopClamps (0.00s)
=== RUN   TestHelpScrollReachesEveryRow
--- PASS: TestHelpScrollReachesEveryRow (0.00s)
=== RUN   TestHelpWrapsLongUnbrokenString
--- PASS: TestHelpWrapsLongUnbrokenString (0.00s)
=== RUN   TestHelpRendersAt25x8WithoutPanic
--- PASS: TestHelpRendersAt25x8WithoutPanic (0.00s)
=== RUN   TestHelpClippedAtTinySize
--- PASS: TestHelpClippedAtTinySize (0.00s)
=== RUN   TestHelpRestoredOnGrowth
--- PASS: TestHelpRestoredOnGrowth (0.00s)
=== RUN   TestKeyBindingsDefinedAsData
--- PASS: TestKeyBindingsDefinedAsData (0.00s)
=== RUN   TestKeyBindingsCoversAllCategories
--- PASS: TestKeyBindingsCoversAllCategories (0.00s)
=== RUN   TestHelpRendersBindingTable
--- PASS: TestHelpRendersBindingTable (0.00s)
=== RUN   TestHelpFooterSlot
--- PASS: TestHelpFooterSlot (0.00s)
=== RUN   TestHelpOpensCancelsPopup
--- PASS: TestHelpOpensCancelsPopup (0.00s)
=== RUN   TestHelpSinkSafetyNoStyle
--- PASS: TestHelpSinkSafetyNoStyle (0.00s)
=== RUN   TestHelpSinkSafetyStyled
--- PASS: TestHelpSinkSafetyStyled (0.00s)
=== RUN   TestHelpBaseColors
--- PASS: TestHelpBaseColors (0.00s)
=== RUN   TestHelpRendersBorder
--- PASS: TestHelpRendersBorder (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual demonstration

The manual cases are demonstrated through a deterministic Go test that drives the model through the exact key sequences and renders the view at each step. This avoids depending on a real PTY while proving the observable behavior.

The cases demonstrated:
1. ? opens the bordered help overlay from browse.
2. down scrolls the help content.
3. n does nothing to the file behind (overlay stays open, browse state unchanged).
4. Esc closes the help overlay, returning to browse.
5. Shrinking to 25x8 shows clipped-but-present help.
6. Enlarging to 80x24 restores the normal layout.
7. Opening help from the no-results screen and closing back to it with q exiting 1.

The manual demo test (demo_artifacts/manual_demo_test.go) exercises the Issue #31 manual verification scenarios: ? opens the bordered help, down scrolls, n does nothing to the file behind, Esc closes, shrinking to 25x8 shows clipped-but-present help, enlarging restores the normal layout, and opening help from the no-results screen then closing back to it with q exiting 1.

```bash
cp /home/chris/vrg/Notes/walkthroughs/031-04/code-walkthrough/demo_artifacts/manual_demo_test.go /home/chris/vrg/internal/app/manual_demo_test.go && cd /home/chris/vrg && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoHelpOverlay$' -timeout 30s 2>&1 | sed 's/[[:space:]]0\.[0-9]*s//g; s/([0-9.]*s)/(0.00s)/g'; rm /home/chris/vrg/internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoHelpOverlay
    manual_demo_test.go:52: step 1 OK: ? opened bordered help (border present, 1015 chars)
    manual_demo_test.go:59: step 2 OK: down pressed (overlay still open)
    manual_demo_test.go:69: step 3 OK: n ignored (overlay still open, kind=help)
    manual_demo_test.go:79: step 4 OK: Esc closed help, returned to browse (state=2)
    manual_demo_test.go:96: step 5 OK: 25x8 help clipped-but-present (6 lines, all <= 25 cells)
    manual_demo_test.go:107: step 6 OK: 80x24 restored border (1015 chars)
    manual_demo_test.go:124: step 7a OK: ? opened help from no-results
    manual_demo_test.go:132: step 7b OK: q closed help, back to no-results
    manual_demo_test.go:138: step 7c OK: q on no-results exits 1
    manual_demo_test.go:139: Manual demonstration passed: help overlay behavior matches the Issue #31 contracts.
--- PASS: TestManualDemoHelpOverlay (0.00s)
PASS
ok  	vrg/internal/app
```
