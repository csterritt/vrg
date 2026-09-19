# Issue #7: theme — c colour toggle, inverse matches, underlines

*2026-09-16T21:15:35Z by Showboat 0.6.1*
<!-- showboat-id: a326e487-ca21-4e95-84c0-2840a344790d -->

Issue #7 lands the full Theme module: the dark (white on black) and light (black on white) schemes with the session-only `c` toggle, true-inverse match colours, underlined current-matched-line matches and current file-list entry, inverse indicators, and the base-colour single-line-bordered overlay style — all supplied by `internal/theme` and consumed by the browse renderer. See `Notes/issues/007-theme-colour-toggle-and-match-styles.md`, `Notes/tasks/007-theme-colour-toggle-and-match-styles.md`, and the PRD sections "Colours, overlays, and key precedence" and "Module Design → Theme" in `Notes/PRD-vrg.md`. All artifacts live in this directory.

## Gates — module integrity, build, vet

```bash
cd /home/chris/vrg && go mod verify && go build ./... && go vet ./... && echo GATES-OK
```

```output
all modules verified
GATES-OK
```

## Full suite — `go test ./...`

```bash
cd /home/chris/vrg && go test -count=1 ./... 2>&1 | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
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
?   	vrg/internal/viewport	[no test files]
```

## Theme unit tests — both schemes, inverse, underlines, overlay

`internal/theme/theme_test.go` pins the contracts: dark is white on black (`37;40`) and initially active, light is black on white (`30;47`); `Toggled` is a pure dark↔light value transform with nothing persisted; `Match` renders the true inverse of the active scheme — the dark match pair is the light base pair and vice versa; `CurrentMatch` adds underline; `Indicator` shares the inverse style; `CurrentFile` underlines the current file-list entry while `FileList`/`Gutter`/`FilenameRule` stay base; `Overlay` frames rows in a plain single-line border in base colours; and `Plain()` stays the no-style path.

```bash
cd /home/chris/vrg && go test -count=1 -v ./internal/theme 2>&1 | grep -E '^(--- |    --- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestSchemeColourPairs
    --- PASS: TestSchemeColourPairs/dark
    --- PASS: TestSchemeColourPairs/light
--- PASS: TestToggleFlipsSchemes
--- PASS: TestMatchIsTrueInverse
    --- PASS: TestMatchIsTrueInverse/dark
    --- PASS: TestMatchIsTrueInverse/light
--- PASS: TestCurrentMatchUnderlines
    --- PASS: TestCurrentMatchUnderlines/dark
    --- PASS: TestCurrentMatchUnderlines/light
--- PASS: TestIndicatorIsInverse
--- PASS: TestCurrentFileUnderlined
    --- PASS: TestCurrentFileUnderlined/dark
    --- PASS: TestCurrentFileUnderlined/light
--- PASS: TestBaseColourStyles
    --- PASS: TestBaseColourStyles/dark
    --- PASS: TestBaseColourStyles/light
--- PASS: TestOverlayBorderBaseColours
--- PASS: TestPlainNoStyle
ok  	vrg/internal/theme
```

## App model — `c` flips the composed view

`TestCTogglesColourScheme` drives a real keypress through `Update`: the session opens dark (the composed frame starts `\x1b[37;40m`), `c` repaints it light (`\x1b[30;47m`) with the current matched line's match still inverse and underlined — now in the other scheme's colours — and a second `c` restores the dark frame byte-for-byte.

```bash
cd /home/chris/vrg && go test -count=1 -v -run 'TestCTogglesColourScheme|TestLoadCompletionRendersContent|TestCurrentFileListEntryUnderlined|TestGutterRightJustified' ./internal/app 2>&1 | grep -E '^(--- |ok|FAIL)' | sed -E 's/[[:space:]]*\(?[0-9.]+s\)?$//'
```

```output
--- PASS: TestLoadCompletionRendersContent
--- PASS: TestCurrentFileListEntryUnderlined
--- PASS: TestGutterRightJustified
--- PASS: TestCTogglesColourScheme
ok  	vrg/internal/app
```

## Manual check — `vrg func .` on a pty, pressing `c`

Build the real binary into this directory, then `pty_theme.py` runs it on a pty against `fixture/` (a small Go tree with `func` matches), waits for the browse view, sends `c`, `c`, `q`, and asserts on the raw byte stream a terminal would execute: the dark frame is white on black with the current matched line's match in true inverse + underline and the current file-list entry underlined; `c` swaps to black on white with matches still inverse and still underlined on the current line; the second `c` restores dark; `q` exits 0 after leaving the alt screen.

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/007-04/code-walkthrough/vrg ./cmd/vrg && python3 Notes/walkthroughs/007-04/code-walkthrough/pty_theme.py
```

```output
dark   : white on black (37;40); current-line match 30;47;4mfunc, other match 30;47m, list entry underlined
light  : c swaps to black on white (30;47); current-line match 37;40;4mfunc, other match 37;40m, entry underlined
dark2  : second c returns to white on black
quit   : q exits 0, alt screen restored
OK
```

All Issue #7 contracts verified: the dark scheme is initially active as white on black, `c` toggles to light and back with no persistence, matches render the true inverse of the active scheme's base colours with the current matched line additionally underlined, the current file-list entry is underlined in both schemes, and the overlay style supplies base colours inside a plain single-line border for Issues #9/#15/#31 to consume.
