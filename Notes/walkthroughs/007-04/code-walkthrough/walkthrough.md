# Issue #7: Theme — c colour toggle, inverse matches, current-line underline

*2026-09-23T19:01:02Z by Showboat 0.6.1*
<!-- showboat-id: ed9f9483-70f8-493c-806d-82dc3eb9dd8e -->

Walkthrough for [Issue #7](../../../issues/007-theme-colour-toggle-and-match-styles.md), implementing the Theme module and its wiring per `Notes/PRD-vrg.md` (*Colours, overlays, and key precedence* — first bullet; *Module Design → Theme*). The dark scheme (white on black, SGR `37;40`) is initially active; `c` toggles to light (black on white, `30;47`) and back with no persistence. Matches render in the true inverse of the active scheme's base pair — explicit swapped pairs, not bare SGR 7 — with an added underline on the current matched line (the first stop until Issue #13); the current file-list entry is underlined in both schemes; indicators use the inverse pair; overlays take base colours with a plain single-line border. All generated artifacts live in this directory: the built `vrg` binary, `demo-theme.sh`, and its `manual-theme/` session files. Test durations are stripped so the document verifies cleanly.

```bash
cd /home/chris/vrg && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && echo GATES-OK
```

```output
ok  	vrg/cmd/vrg
ok  	vrg/internal/app
ok  	vrg/internal/cli
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
ok  	vrg/internal/searchindex
ok  	vrg/internal/theme
?   	vrg/internal/viewport	[no test files]
GATES-OK
```

## Theme unit tests

`internal/theme/theme_test.go` pins every Issue #7 contract at exact-SGR granularity: the dark scheme is initially active (white `37` on black `40`), `Toggle` round-trips dark → light → dark as in-memory state with no persistence, `Match`/`Indicator` emit the scheme's true inverse pair and restore the base pair, `CurrentMatch` adds underline to the inverse pair, `CurrentFile` is underline-only in both schemes, `Gutter`/`FileList`/`FilenameRule` emit the base pair, `Overlay` frames content in a plain single-line border in the base colours, and `Plain` keeps every style the identity so the sink-safety table's no-escape-byte method still holds.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/theme 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestDarkSchemeIsInitiallyActive
--- PASS: TestSchemeColourPairs
--- PASS: TestToggleRoundTrip
--- PASS: TestMatchIsTrueInverse
--- PASS: TestCurrentMatchUnderline
--- PASS: TestCurrentFileUnderline
--- PASS: TestIndicatorInverse
--- PASS: TestBaseColourStyles
--- PASS: TestOverlayStyle
--- PASS: TestPlainIsIdentity
PASS
ok  	vrg/internal/theme
```

## App toggle and current-line tests

In `internal/app`, `TestColourToggleFlipsViewStyling` sends `c` through `Update` and asserts the composed `View()` frame's first SGR sequence — the `theme.Base` wrap — flips `37;40` → `30;47` → `37;40`, with `c` returning no command and quitting nothing. `TestCurrentLineMatchUnderlined` distinguishes the underlined inverse match on the current matched line from a plain-inverse match on another matched line. The Issue #5 assertions moved from bare SGR 7 to the explicit inverse pairs (`TestMatchRendersInverse`, `TestBrowseRendering`).

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestColourToggleFlipsViewStyling|TestCurrentLineMatchUnderlined|TestMatchRendersInverse|TestBrowseRendering" ./internal/app 2>&1 | grep -vE "^(=== RUN|    --- (PASS|FAIL))" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestMatchRendersInverse
--- PASS: TestBrowseRendering
--- PASS: TestColourToggleFlipsViewStyling
--- PASS: TestCurrentLineMatchUnderlined
PASS
ok  	vrg/internal/app
```

## Manual check — `c` swaps the schemes on a real PTY

`demo-theme.sh` (checked into this directory) drives the `vrg` binary above on a real PTY via `script(1)`: an 80x24 inner session runs `vrg func .` in a Go fixture whose single matched line is the current stop — bare `vrg` prints command-line help and exits, so a search invocation is required to demonstrate the toggle. The harness waits for the dark browse frame, sends `c`, waits for the light frame, sends `c` again, then `q`. Byte offsets into the captured typescript separate the three phases, and each phase is checked for its discriminator sequences: dark paints base `37;40` and the current-line match `30;47;4` (a bare `30;47m` never occurs), light paints base `30;47` and the match `37;40;4` (a bare `37;40m` never occurs). The terminal renderer canonicalizes embedded SGR per cell, so the underlined current file-list entry arrives merged as `37;40;4`/`30;47;4` rather than a bare `4`.

```bash
cd /home/chris/vrg/Notes/walkthroughs/007-04/code-walkthrough && ./demo-theme.sh
```

```output
exit=0
dark-base=present
dark-match-inv-uline=present
dark-current-file-uline=present
dark-no-light-base=absent
light-base=present
light-match-inv-uline=present
light-current-file=present
light-no-dark-base=absent
dark-base-again=present
dark-match-again=present
dark-no-light-base=absent
--- rendered rows ---
./main.go  ── ./main.go ────────────────────────────────────────────────────────
 1  package main
 2

 3  func main() {
 4  →println("hi")
 5  }
./main.go  ── ./main.go ────────────────────────────────────────────────────────
```

## Result

Issue #7 is verified. `internal/theme` owns the active colour scheme — dark initially (white on black), `c` toggling to light (black on white) and back with no persistence — and supplies the full style set: base, gutter, true-inverse match, underlined current match, inverse indicator, base-coloured single-line-bordered overlay, filename rule, file list, and current-file underline. Rendering consumes them throughout the browse composition, and the live PTY run shows the foreground/background swap on `c` with the match staying inverse and underlined, then returning on the second `c`; the process exits 0 on `q`. Deferred by design: real match navigation beyond the first stop (Issue #13), the indicator consumers (Issue #20), and the overlay consumers (#9, #15, #31).
