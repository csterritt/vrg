# Issue #31: the h/? help overlay — modal, wrapped, scrollable key bindings

*2026-09-18T01:32:52Z by Showboat 0.6.1*
<!-- showboat-id: 972a3bb8-2fc9-4847-82d8-40354a6cd0f9 -->

Issue #31 lands the help overlay: 'h' and '?' open a modal help dialog over ordinary browsing and the no-results screen — a bordered, wrapped, scrollable box listing every key binding. The implementation factors the Issue #9 error overlay's body into a shared scrollBox component (grapheme-boundary wrapping that splits unbroken strings, scroll clamped to the complete row set) composited by compositeBox through theme.Overlay's single-line border — at tiny sizes the box clips to the terminal with no borderless fallback, and growth restores the layout. While open, up/down scroll, q/Esc/h/? close, ctrl+c keeps the global 130 override, and every other key — n, p, w, c, r included — is ignored with the state behind untouched; opening help cancels a live Issue #15 file-change pop-up that never returns. helpBindings is the single binding-table data source the renderer consumes and Issue #34's documentation tests will iterate, with a reserved helpFooter slot routed through safepresentation.EscapeDiagnostic so its substitution can never emit control bytes — the sink-safety table's new 'TUI help dialog' row drives the hostile fixtures through it. Precedence: ctrl+c over the error overlay over help over the pop-up over base keys; an error opening over help suspends it at its retained scroll. See Notes/issues/031-help-overlay.md, Notes/tasks/031-help-overlay.md, and the 'Colours, overlays, and key precedence' section of Notes/PRD-vrg.md. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Model tests — the Issue #31 help contracts

internal/app/help_test.go pins the whole contract: 'h' and '?' open help from ordinary browse and from the no-results screen; q, Esc, 'h', and '?' each close back to the underlying base state — with a subsequent q on no-results still exiting 1; up/down scroll the complete wrapped table clamped at both ends; ctrl+c exits 130; the ignored-keys sweep (n, p, w, c, r, and the rest) leaves cursor, viewport, wrap, theme, list, and loading untouched; a 200-cell unbroken footer substitution wraps within the single-line border; the 25x8 frame clips to the terminal, still bordered with no borderless fallback, and growth restores the layout; every helpBindings row's keys and description reach the render; and a live file-change pop-up is cancelled on open and never returns. The sink-safety table's new 'TUI help dialog' row drives the hostile fixture set through helpFixtureView — the footer substitution routed through EscapeDiagnostic.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestHelp|TestSinkSafetyTable/TUI_help_dialog' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL|    ---)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestHelpOpensFromBrowseAndCloses
--- PASS: TestHelpOpensFromNoResults
--- PASS: TestHelpIgnoresBaseKeys
--- PASS: TestHelpScrollsRenderedRows
--- PASS: TestHelpCtrlCExits130
--- PASS: TestHelpWrapsUnbrokenSubstitution
--- PASS: TestHelpClippedAtTinySize
--- PASS: TestHelpRendersBindingTable
--- PASS: TestHelpCancelsPopup
--- PASS: TestSinkSafetyTable
    --- PASS: TestSinkSafetyTable/TUI_help_dialog/osc_title-set
    --- PASS: TestSinkSafetyTable/TUI_help_dialog/csi_erase
    --- PASS: TestSinkSafetyTable/TUI_help_dialog/c0_run
    --- PASS: TestSinkSafetyTable/TUI_help_dialog/c1_nel
    --- PASS: TestSinkSafetyTable/TUI_help_dialog/del
    --- PASS: TestSinkSafetyTable/TUI_help_dialog/standalone_cr
    --- PASS: TestSinkSafetyTable/TUI_help_dialog/invalid_utf-8
    --- PASS: TestSinkSafetyTable/TUI_help_dialog/embedded_filename_newline
ok  	vrg/internal/app
```

## Full module regression

The refactor touched the shared overlay component every overlay consumes — the error overlay, the file-change pop-up, and the new help dialog — plus the key routing and view composition, so the whole suite runs, plus the race detector over internal/app.

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

manual_route.sh runs the issue's manual route against a disposable mktemp fixture — never a repository file: aa.txt and bb.txt with 'foo' matches, under a trap removing the tree on exit or interruption. pty_help.py drives two sessions on a real 60x14 pty. Session one ('vrg foo .'): '?' opens the bordered 'Key bindings' box over the browse view; 'down' x4 scrolls the complete table to 'ctrl+c' with the title row scrolled off; 'n' is ignored — the frame behind help is byte-identical; Esc closes back to browse. 'h' reopens, the pty shrinks to 25x8 and help clips to the terminal — still bordered, no borderless fallback, no panic; enlarging to 60x14 restores the normal layout; Esc and q exit 0. Session two ('vrg <no-match> .'): '?' opens help over 'No results found', Esc returns to it, and q exits 1.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/031-04/code-walkthrough/vrg ./cmd/vrg && bash Notes/walkthroughs/031-04/code-walkthrough/manual_route.sh
```

```output
fixture: <tmp> (aa.txt, bb.txt — 'foo' matches)
?           : bordered 'Key bindings' over the browse view
down x4     : table scrolled — 'ctrl+c' in, title out
n           : ignored — the file behind help is unchanged
Esc         : help closed — the browse view is back
25x8        : clipped to the terminal, still bordered
60x14       : enlarged — the normal layout is back
Esc         : help closed — the browse view is back
q           : exit 0
?           : bordered help over 'No results found'
Esc         : help closed — 'No results found' is back
q           : exit 1
OK
```

Artifacts in this directory: vrg (the built binary the pty sessions ran), manual_route.sh (the disposable-fixture driver), pty_help.py (the two-session pty emulator and assertions — including DECSTBM-region scrolling and ECH handling, which is how vrg's differential renderer repaints scrolled overlays).
