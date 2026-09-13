# Issue #7: Theme colour toggle and match styles

*2026-09-11T23:50:04Z by Showboat 0.6.1*
<!-- showboat-id: 0659a699-50c4-4192-9832-dc8387931b5d -->

Walkthrough for Issue #7 (Notes/tasks/007-theme-colour-toggle-and-match-styles.md), implementing the theme system with dark/light colour schemes, the `c` toggle, true-inverse match styling, current-match underline, current-file underline, and overlay styling. References: Notes/PRD-vrg.md (Colours, overlays, and key precedence, Module Design → Theme).

Contracts verified:
- The dark scheme is initially active (white on black, ANSI 37;40).
- The `c` key toggles to the light scheme (black on white, ANSI 30;47).
- `c` toggles back to dark; the toggle has no persistence.
- Matches use the true inverse of the active scheme base colours (dark: 30;47, light: 37;40), not SGR 7 reverse video.
- The current matched line adds underline to the true-inverse match (dark: 30;47;4m, light: 37;40;4m).
- The current file-list entry is underlined in both schemes.
- Overlay style uses base colours with a plain single-line border.
- The no-style theme produces no ANSI sequences (sink-safety).
- App-level `c` keypress changes the composed `View()` styling between schemes.

All generated artifacts live in this directory.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./... -timeout 60s | sed 's/[[:space:]][0-9.]*s$//'
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
?   	vrg/internal/viewport	[no test files]
```

## Theme unit tests

The theme unit tests (internal/theme/theme_test.go) verify both colour schemes, the toggle, true-inverse matches, current-match underline, current-file underline, overlay styling, and no-style passthrough.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/theme/ -timeout 30s
```

```output
=== RUN   TestNewStartsDark
--- PASS: TestNewStartsDark (0.00s)
=== RUN   TestToggleDarkToLight
--- PASS: TestToggleDarkToLight (0.00s)
=== RUN   TestToggleLightToDark
--- PASS: TestToggleLightToDark (0.00s)
=== RUN   TestToggleNoPersistence
--- PASS: TestToggleNoPersistence (0.00s)
=== RUN   TestNoStyleScheme
--- PASS: TestNoStyleScheme (0.00s)
=== RUN   TestNoStyleToggleIsNoOp
--- PASS: TestNoStyleToggleIsNoOp (0.00s)
=== RUN   TestDarkBaseColours
--- PASS: TestDarkBaseColours (0.00s)
=== RUN   TestLightBaseColours
--- PASS: TestLightBaseColours (0.00s)
=== RUN   TestBaseResets
--- PASS: TestBaseResets (0.00s)
=== RUN   TestDarkMatchTrueInverse
--- PASS: TestDarkMatchTrueInverse (0.00s)
=== RUN   TestLightMatchTrueInverse
--- PASS: TestLightMatchTrueInverse (0.00s)
=== RUN   TestMatchRestoresBase
--- PASS: TestMatchRestoresBase (0.00s)
=== RUN   TestDarkCurrentMatchUnderline
--- PASS: TestDarkCurrentMatchUnderline (0.00s)
=== RUN   TestLightCurrentMatchUnderline
--- PASS: TestLightCurrentMatchUnderline (0.00s)
=== RUN   TestCurrentMatchRestoresBase
--- PASS: TestCurrentMatchRestoresBase (0.00s)
=== RUN   TestDarkIndicatorInverse
--- PASS: TestDarkIndicatorInverse (0.00s)
=== RUN   TestLightIndicatorInverse
--- PASS: TestLightIndicatorInverse (0.00s)
=== RUN   TestUnderlineSequence
--- PASS: TestUnderlineSequence (0.00s)
=== RUN   TestUnderlineRestoresBase
--- PASS: TestUnderlineRestoresBase (0.00s)
=== RUN   TestUnderlineBothSchemes
--- PASS: TestUnderlineBothSchemes (0.00s)
=== RUN   TestGutterUsesBaseColours
--- PASS: TestGutterUsesBaseColours (0.00s)
=== RUN   TestFileListUsesBaseColours
--- PASS: TestFileListUsesBaseColours (0.00s)
=== RUN   TestFilenameRuleEmbedsName
--- PASS: TestFilenameRuleEmbedsName (0.00s)
=== RUN   TestFilenameRuleUsesBaseColours
--- PASS: TestFilenameRuleUsesBaseColours (0.00s)
=== RUN   TestOverlayUsesBaseColours
--- PASS: TestOverlayUsesBaseColours (0.00s)
=== RUN   TestOverlaySingleLineBorder
--- PASS: TestOverlaySingleLineBorder (0.00s)
=== RUN   TestOverlayContainsContent
--- PASS: TestOverlayContainsContent (0.00s)
=== RUN   TestNoStyleBaseIsPassthrough
--- PASS: TestNoStyleBaseIsPassthrough (0.00s)
=== RUN   TestNoStyleMatchIsPassthrough
--- PASS: TestNoStyleMatchIsPassthrough (0.00s)
=== RUN   TestNoStyleCurrentMatchIsPassthrough
--- PASS: TestNoStyleCurrentMatchIsPassthrough (0.00s)
=== RUN   TestNoStyleIndicatorIsPassthrough
--- PASS: TestNoStyleIndicatorIsPassthrough (0.00s)
=== RUN   TestNoStyleUnderlineIsPassthrough
--- PASS: TestNoStyleUnderlineIsPassthrough (0.00s)
=== RUN   TestNoStyleGutterIsPassthrough
--- PASS: TestNoStyleGutterIsPassthrough (0.00s)
=== RUN   TestNoStyleFileListIsPassthrough
--- PASS: TestNoStyleFileListIsPassthrough (0.00s)
=== RUN   TestNoStyleFilenameRuleIsPassthrough
--- PASS: TestNoStyleFilenameRuleIsPassthrough (0.00s)
=== RUN   TestNoStyleOverlayIsPassthrough
--- PASS: TestNoStyleOverlayIsPassthrough (0.00s)
=== RUN   TestNoStyleProducesNoANSI
--- PASS: TestNoStyleProducesNoANSI (0.00s)
PASS
ok  	vrg/internal/theme	0.002s
```

## App theme toggle tests

The App tests (internal/app/browse_test.go, Issue #7) verify that pressing `c` in the browse state toggles the theme between dark and light, changing the composed `View()` styling. The tests confirm: dark starts active (white on black, 37;40), `c` toggles to light (black on white, 30;47), `c` again returns to dark, the toggle has no persistence, `c` does not quit, matches use true-inverse colours, and the current matched line adds underline.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/app/ -run 'TestBrowseC|TestBrowseMatchTrueInverse|TestBrowseCurrentMatchUnderline' -timeout 30s
```

```output
=== RUN   TestBrowseCtrlCExits130
--- PASS: TestBrowseCtrlCExits130 (0.00s)
=== RUN   TestBrowseCurrentFileUnderlined
--- PASS: TestBrowseCurrentFileUnderlined (0.00s)
=== RUN   TestBrowseCToggleThemeDarkToLight
--- PASS: TestBrowseCToggleThemeDarkToLight (0.00s)
=== RUN   TestBrowseCToggleThemeLightToDark
--- PASS: TestBrowseCToggleThemeLightToDark (0.00s)
=== RUN   TestBrowseCToggleNoPersistence
--- PASS: TestBrowseCToggleNoPersistence (0.00s)
=== RUN   TestBrowseCDoesNotQuit
--- PASS: TestBrowseCDoesNotQuit (0.00s)
=== RUN   TestBrowseMatchTrueInverseDark
--- PASS: TestBrowseMatchTrueInverseDark (0.00s)
=== RUN   TestBrowseMatchTrueInverseLight
--- PASS: TestBrowseMatchTrueInverseLight (0.00s)
=== RUN   TestBrowseCurrentMatchUnderlineDark
--- PASS: TestBrowseCurrentMatchUnderlineDark (0.00s)
=== RUN   TestBrowseCurrentMatchUnderlineLight
--- PASS: TestBrowseCurrentMatchUnderlineLight (0.00s)
PASS
ok  	vrg/internal/app	0.024s
```

## Binary demo: `vrg func .` with `c` toggle

Using the built vrg binary with a fake rg that emits JSON results for two files (fixtures/src/alpha.txt and fixtures/src/zebra.txt). The PTY helper (runpty.py) waits for output to settle, then sends keys. The stripped output shows the file list on the left (alpha underlined as the current file) and the content panel on the right (filename rule, gutter, content with matches).

```bash
cd /home/chris/vrg/Notes/walkthroughs/007-04/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=1.0 timeout 10 python3 runpty.py ./vrg hello fixtures 2>&1; echo exit=$?
```

```output
fixtures/src/alpha.txt    ── fixtures/src/alpha.txt ──
fixtures/src/zebra.txt    1  hello world 2  line 2 3  foo bar baz
exit=0
```

The raw PTY output (before ANSI stripping) shows the dark scheme styling: the current file (alpha.txt) is underlined with the dark base colours (\\\\x1b[37;40;4m), non-current files use the dark base (\\\\x1b[37;40m), the current matched line (line 1) uses the true-inverse match with underline (\\\\x1b[30;47;4m), and non-current matches use the true-inverse match (\\\\x1b[30;47m). After each match, the base colours are restored (\\\\x1b[37;40m).

```bash
cd /home/chris/vrg/Notes/walkthroughs/007-04/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=q VRG_DELAY=1.0 VRG_RAW=1 timeout 10 python3 runpty.py ./vrg hello fixtures 2>&1 | cat -v
```

```output
^[[>4m^[[?1049h^[[?25l^[[?5W^[[?2004h^[[>4;2m^[[>1u^[[?u^[[H^[[2J^[[37;40;4mfixtures/src/alpha.txt    M-bM-^TM-^@M-bM-^TM-^@ fixtures/src/alpha.txt M-bM-^TM-^@M-bM-^TM-^@^[[m^M
^[[37;40mfixtures/src/zebra.txt    1  ^[[30;47;4mhello^[[37;40m world^[[m^[[3;26H^[[37;40m^[[1K 2  line 2^[[m^[[4;26H^[[37;40m^[[1K 3  ^[[30;47mfoo^[[37;40m bar baz^[[m^[[5;26H^[[37;40m^[[1K^[[m^[[>4m^[[<1u^M^[[30d^[[?1049l^[[?25h^[[?2004l
```

````output
^[[>4m^[[?1049h^[[?25l^[[?5W^[[?2004h^[[>4;2m^[[>1u^[[?u^[[H^[[2J^[[37;40;4mfixtures/src/alpha.txt    M-bM-^TM-^@M-bM-^TM-^@ fixtures/src/alpha.txt M-bM-^TM-^@M-bM-^TM-^@^[[m^M
^[[37;40mfixtures/src/zebra.txt    1  ^[[30;47;4mhello^[[37;40m world^[[m^[[3;26H^[[37;40m^[[1K 2  line 2^[[m^[[4;26H^[[37;40m^[[1K 3  ^[[30;47mfoo^[[37;40m bar baz^[[m^[[5;26H^[[37;40m^[[1K^[[m^[[H^[[30;47;4mfixtures/src/alpha.txt    M-bM-^TM-^@M-bM-^TM-^@ fixtures/src/alpha.txt M-bM-^TM-^@M-bM-^TM-^@^[[m^M
^[[30;47mfixtures/src/zebra.txt    1  ^[[37;40;4mhello^[[30;47m world^[[m^[[3;26H^[[30;47m^[[1K 2  line 2^[[m^[[4;26H^[[30;47m^[[1K 3  ^[[37;40mfoo^[[30;47m bar baz^[[m^[[5;26H^[[30;47m^[[1K^[[m^[[>4m^[[<1u^M^[[30d^[[?1049l^[[?25h^[[?2004l```

## Pressing `c` again to return to dark

Pressing `c` again toggles back to the dark scheme. The raw PTY output shows three frames: the initial dark frame, the light frame after the first `c`, and the dark frame after the second `c`. The third frame uses the same dark sequences as the first (\\\\x1b[37;40;4m for the current file, \\x1b[30;47;4m for the current match), confirming the toggle returns to the original scheme with no persistence.

```bash
cd /home/chris/vrg/Notes/walkthroughs/007-04/code-walkthrough && PATH=$(pwd)/fakebin:/usr/bin:/bin VRG_KEYS=c,c,q VRG_DELAY=1.0 VRG_RAW=1 timeout 10 python3 runpty.py ./vrg hello fixtures 2>&1 | cat -v
````

```output
^[[>4m^[[?1049h^[[?25l^[[?5W^[[?2004h^[[>4;2m^[[>1u^[[?u^[[H^[[2J^[[37;40;4mfixtures/src/alpha.txt    M-bM-^TM-^@M-bM-^TM-^@ fixtures/src/alpha.txt M-bM-^TM-^@M-bM-^TM-^@^[[m^M
^[[37;40mfixtures/src/zebra.txt    1  ^[[30;47;4mhello^[[37;40m world^[[m^[[3;26H^[[37;40m^[[1K 2  line 2^[[m^[[4;26H^[[37;40m^[[1K 3  ^[[30;47mfoo^[[37;40m bar baz^[[m^[[5;26H^[[37;40m^[[1K^[[m^[[H^[[30;47;4mfixtures/src/alpha.txt    M-bM-^TM-^@M-bM-^TM-^@ fixtures/src/alpha.txt M-bM-^TM-^@M-bM-^TM-^@^[[m^M
^[[30;47mfixtures/src/zebra.txt    1  ^[[37;40;4mhello^[[30;47m world^[[m^[[3;26H^[[30;47m^[[1K 2  line 2^[[m^[[4;26H^[[30;47m^[[1K 3  ^[[37;40mfoo^[[30;47m bar baz^[[m^[[5;26H^[[30;47m^[[1K^[[m^[[H^[[37;40;4mfixtures/src/alpha.txt    M-bM-^TM-^@M-bM-^TM-^@ fixtures/src/alpha.txt M-bM-^TM-^@M-bM-^TM-^@^[[m^M
^[[37;40mfixtures/src/zebra.txt    1  ^[[30;47;4mhello^[[37;40m world^[[m^[[3;26H^[[37;40m^[[1K 2  line 2^[[m^[[4;26H^[[37;40m^[[1K 3  ^[[30;47mfoo^[[37;40m bar baz^[[m^[[5;26H^[[37;40m^[[1K^[[m^[[>4m^[[<1u^M^[[30d^[[?1049l^[[?25h^[[?2004l```

## Summary

The walkthrough demonstrates:
- Theme unit tests for both schemes (dark: white on black, light: black on white), toggle, true-inverse matches, current-match underline, current-file underline, overlay styling, and no-style passthrough.
- App toggle tests verifying `c` toggles the composed `View()` styling between schemes with no persistence and without quitting.
- The binary with a real search invocation (`vrg hello fixtures`), showing the dark scheme initially active.
- Pressing `c` swaps foreground/background colours (dark → light); matches remain inverse; current-line matches remain underlined.
- Pressing `c` again returns to the original dark scheme.

References: Issue #7 (Notes/tasks/007-theme-colour-toggle-and-match-styles.md), Notes/PRD-vrg.md (Colours, overlays, and key precedence; Module Design → Theme).
```
