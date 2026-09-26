# Issue #31: Help overlay — h/? modal key bindings

*2026-09-24T19:02:34Z by Showboat 0.6.1*
<!-- showboat-id: 757decea-8562-4eca-9d59-8b4bcc1ce649 -->

Walkthrough for [Issue #31](../../../issues/031-help-overlay.md), implementing the modal help overlay per `Notes/PRD-vrg.md` (*Colours, overlays, and key precedence* — the help bullets): `h` and `?` open a bordered, scrollable help box over ordinary browsing and the no-results screen; while open, `up`/`down` scroll the rendered rows, `q`/`Esc`/`h`/`?` close back to the underlying base state, `ctrl+c` exits 130, and every other key is ignored — nothing reaches the content behind. The Issue #9 diagnostics overlay and the help overlay are two instances of one shared wrapped-scrollable component in `internal/app/overlay.go`, so wrapping, scroll clamping, and centred clipping behave identically; the binding list is the single `helpBindings` table consumed by the renderer now and by Issue #34's documentation tests later, with the `helpFooter` substitution slot — routed through `present.Diagnostic` — reserved for Issue #34's scale-and-limits note. Opening help cancels a live file-change pop-up with no return, and an error arriving over help suspends it at its retained scroll position for Issue #32's combined precedence. All generated artifacts live in this directory: the built `vrg` binary, the `demo-help.sh` tmux harness, and its `help/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/ cmd/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/031-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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

## Model tests — `internal/app/help_test.go` + the sink table

The twelve `help_test.go` tests pin the modal contract through `Update` alone — no sleeps. `TestHelpOpensFromBrowse` and `TestHelpOpensFromNoResults` open help with `h` and `?` over each base state; `TestHelpCloseKeys` tables all four close keys back to the state behind, including the `q`-then-`q` route that still exits no-results at status 1. `TestHelpIgnoresOtherKeys` walks `n`, `p`, `w`, `c`, `r`, and an unrelated key under an open help and asserts the model behind is untouched; `TestHelpCtrlCExits130` takes the cancellation path; `TestHelpKeysInertWhileSearching` keeps `h`/`?` inert mid-search like every other ordinary key. `TestHelpScrollsWithUpDown` clamps the offset at both ends and reaches every rendered row; `TestHelpWrapsUnbrokenSubstitution` hard-wraps a run of unbroken footer text to the interior width; `TestHelpClippedAtTinySize` renders 25×8 without panic, keeps the border, clips to the terminal, and restores on growth. `TestHelpRendersBindingTable` asserts the `helpBindings` rows land verbatim — the single data source both the renderer and Issue #34's documentation tests consume — and `TestHelpRendersFooterSlot` fills the reserved slot. `TestHelpCancelsPopup` opens help over a live file-change pop-up and proves it never returns after close. The `help overlay` row in `TestSinkSafetyTable` drives the hostile fixtures through the footer substitution into the real themed render, asserting the escaped form inside the border and no raw control bytes.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestHelpOpensFromBrowse|TestHelpOpensFromNoResults|TestHelpCloseKeys|TestHelpIgnoresOtherKeys|TestHelpCtrlCExits130|TestHelpKeysInertWhileSearching|TestHelpScrollsWithUpDown|TestHelpWrapsUnbrokenSubstitution|TestHelpClippedAtTinySize|TestHelpRendersBindingTable|TestHelpRendersFooterSlot|TestHelpCancelsPopup|TestSinkSafetyTable/help_overlay" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestHelpOpensFromBrowse
    --- PASS: TestHelpOpensFromBrowse/h
    --- PASS: TestHelpOpensFromBrowse/?
--- PASS: TestHelpOpensFromNoResults
    --- PASS: TestHelpOpensFromNoResults/h
    --- PASS: TestHelpOpensFromNoResults/?
--- PASS: TestHelpCloseKeys
    --- PASS: TestHelpCloseKeys/q
    --- PASS: TestHelpCloseKeys/esc
    --- PASS: TestHelpCloseKeys/h
    --- PASS: TestHelpCloseKeys/?
--- PASS: TestHelpIgnoresOtherKeys
--- PASS: TestHelpCtrlCExits130
--- PASS: TestHelpKeysInertWhileSearching
--- PASS: TestHelpScrollsWithUpDown
--- PASS: TestHelpWrapsUnbrokenSubstitution
--- PASS: TestHelpClippedAtTinySize
--- PASS: TestHelpRendersBindingTable
--- PASS: TestHelpRendersFooterSlot
--- PASS: TestHelpCancelsPopup
--- PASS: TestSinkSafetyTable
    --- PASS: TestSinkSafetyTable/help_overlay/osc_title_set
    --- PASS: TestSinkSafetyTable/help_overlay/csi_erase
    --- PASS: TestSinkSafetyTable/help_overlay/c0_controls
    --- PASS: TestSinkSafetyTable/help_overlay/c1_nel
    --- PASS: TestSinkSafetyTable/help_overlay/delete
    --- PASS: TestSinkSafetyTable/help_overlay/standalone_carriage_return
    --- PASS: TestSinkSafetyTable/help_overlay/invalid_utf-8_path_bytes
    --- PASS: TestSinkSafetyTable/help_overlay/embedded_filename_newline
PASS
ok  	vrg/internal/app
```

## Manual route — the real binary on a PTY

`demo-help.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with real `rg` over a disposable `mktemp` fixture — repository and user files are never touched, and an EXIT trap removes it. The fixture is `a.txt` and `b.txt` with `hit` lines.

The first session follows the issue's manual route at 80×16 — small enough that the 18-row help scrolls: `?` opens the bordered box, four `down` presses scroll the `ctrl+c` row into view and the title off the head, `n` is ignored (help stays open, and the file behind is untouched — `a.txt` stays current after `Esc`), then `h` reopens and the window is shrunk to 25×8: the box is clipped to the terminal but still bordered — no borderless mode — and growing back to 80×24 restores the normal layout. Two `q` presses close help then quit at the fixed status 0. The second session searches a pattern with no matches: `?` opens help over "No results found", `Esc` returns to it, and `q` exits 1.

```bash
cd /home/chris/vrg/Notes/walkthroughs/031-04/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-help.sh
```

```output
ok: startup: a.txt renders
ok: ?: bordered help opens
ok: ?: single-line border
ok: ?: binding rows listed
ok: ?: ctrl+c row still below the fold
ok: down: ctrl+c row scrolled into view
ok: down: head scrolled off
ok: n under help: help stays open
ok: Esc: help closed
ok: Esc: file behind unchanged (a.txt still current)
ok: Esc: a.txt content intact
ok: 25x8: clipped help still present
ok: 25x8: border still drawn
ok: 25x8: pane rows -> 8
ok: 25x8: every row within 25 cells
ok: 80x24: help restored
ok: 80x24: bottom border restored
ok: q: help closed
ok: vrg exit status -> 0
ok: no-results: screen shown
ok: no-results: ? opens help
ok: no-results: Esc returns to the screen
ok: no-results: help closed
ok: no-results vrg exit status -> 1
demo-help: all checks passed
```

The deterministic model tests remain the authority for the modal key contract, the scroll and wrap semantics, the tiny-size clipping, and the pop-up cancellation; the PTY route demonstrates the visible behavior end to end — the bordered box opening over both base states, the scrolling and ignored keys, the clipped-but-present render at 25×8, the restored layout on growth, and the exit statuses. Wiki ingest lives in `Notes/wiki/help-overlay.md`.
