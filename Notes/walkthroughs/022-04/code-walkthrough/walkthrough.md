# Issue #22: Line terminators, final line, empty file, and the UTF-8 BOM

*2026-09-24T13:48:40Z by Showboat 0.6.1*
<!-- showboat-id: c1987969-c407-4a44-a3f2-933fef0aca49 -->

Walkthrough for [Issue #22](../../../issues/022-line-terminators-final-line-empty-file-utf8-bom.md), implementing FileBuffer structural line handling per `Notes/PRD-vrg.md` (*Text, graphemes, and safe presentation* first two bullets; *Encodings and stale-content validation* UTF-8 BOM bullet): LF and CRLF terminate lines without being displayed while their bytes stay in the raw view for byte-coordinate mapping; a standalone CR escapes as `^M`; a missing final newline yields the final line and a trailing newline invents no phantom one; an empty file has zero source lines behind the minimum one-digit gutter (three cells); removed terminator bytes and zero-width positions map to the display end-of-line position and a span crossing a terminator highlights the visible text only; and a leading UTF-8 BOM is invisible while `Load` maintains separate raw-file and rg-line coordinate views, shifting first-line rg offsets by three into the raw view — non-leading U+FEFF stays ordinary content. All generated artifacts live in this directory: the built `vrg` binary, the `demo-lines.sh` tmux harness, and its `lines/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/)" && echo "GOFMT-CLEAN" && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/viewport ./internal/filebuffer ./internal/present | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/022-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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
ok  	vrg/internal/viewport
ok  	vrg/internal/filebuffer
ok  	vrg/internal/present
GATES-OK
```

## Structural line tests

`internal/filebuffer/lines_test.go` pins the Issue #22 contracts end to end through `Load`: `TestTerminatorsUndisplayed` covers LF, CRLF, and mixed terminators plus the unterminated-final-line and blank-line cases; `TestStandaloneCREscapes` proves a CR not followed by LF is `^M` content, not a terminator; `TestEmptyFile` covers the zero-line file and its one-digit-slot, three-cell gutter; `TestTerminatorBytesMapToDisplayEOL` maps terminator bytes and zero-width positions — byte 4 of `hit\r\n` — to display column 3; `TestSpanAcrossTerminatorHighlightsTextOnly` keeps a text-plus-terminator span on the visible cells; `TestLeadingUTF8BOM`, `TestBOMShiftMapsRGOffsetsToRawBytes`, `TestBOMOnlyFile`, and `TestNonLeadingFEFFIsContent` cover the BOM's invisible display, the rg-offset-0 → raw-byte-3 shift, and U+FEFF as ordinary content everywhere else.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestTerminatorsUndisplayed|TestStandaloneCREscapes|TestEmptyFile|TestTerminatorBytesMapToDisplayEOL|TestSpanAcrossTerminatorHighlightsTextOnly|TestLeadingUTF8BOM|TestBOMShiftMapsRGOffsetsToRawBytes|TestBOMOnlyFile|TestNonLeadingFEFFIsContent" ./internal/filebuffer 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestTerminatorsUndisplayed
    --- PASS: TestTerminatorsUndisplayed/lf
    --- PASS: TestTerminatorsUndisplayed/crlf
    --- PASS: TestTerminatorsUndisplayed/mixed_lf_and_crlf
    --- PASS: TestTerminatorsUndisplayed/crlf_unterminated_final_line
    --- PASS: TestTerminatorsUndisplayed/crlf_blank_line
    --- PASS: TestTerminatorsUndisplayed/lf_blank_line
--- PASS: TestStandaloneCREscapes
--- PASS: TestEmptyFile
--- PASS: TestTerminatorBytesMapToDisplayEOL
    --- PASS: TestTerminatorBytesMapToDisplayEOL/zero-width_at_the_lf_of_a_crlf
    --- PASS: TestTerminatorBytesMapToDisplayEOL/crlf_terminator_only
    --- PASS: TestTerminatorBytesMapToDisplayEOL/cr_byte_of_a_crlf
    --- PASS: TestTerminatorBytesMapToDisplayEOL/lf_terminator_only
    --- PASS: TestTerminatorBytesMapToDisplayEOL/zero-width_at_lf
    --- PASS: TestTerminatorBytesMapToDisplayEOL/zero-width_on_an_empty_line
--- PASS: TestSpanAcrossTerminatorHighlightsTextOnly
    --- PASS: TestSpanAcrossTerminatorHighlightsTextOnly/text_plus_crlf
    --- PASS: TestSpanAcrossTerminatorHighlightsTextOnly/trailing_text_plus_cr
    --- PASS: TestSpanAcrossTerminatorHighlightsTextOnly/text_plus_lf
    --- PASS: TestSpanAcrossTerminatorHighlightsTextOnly/mid-text_through_crlf
--- PASS: TestLeadingUTF8BOM
--- PASS: TestBOMShiftMapsRGOffsetsToRawBytes
--- PASS: TestBOMOnlyFile
    --- PASS: TestBOMOnlyFile/bom_only
    --- PASS: TestBOMOnlyFile/bom_then_lf
--- PASS: TestNonLeadingFEFFIsContent
PASS
ok  	vrg/internal/filebuffer
```

## Manual check — real rg and the fake-rg harness on a real PTY

`demo-lines.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY at 80x24 — the issue's own manual recipe. Run A searches the pattern `hit` with **real rg** over three fixture files: `bom.txt` is a UTF-8-BOM file whose first line is `hit`; `cr.txt` carries a standalone CR mid-line (`a\rb hit`); `crlf.txt` is CRLF-terminated throughout. `n` steps through the three files in raw-path order. Run B uses the fake-rg harness — a `rg` shell script on PATH emitting a valid `begin`/`match`/`end`/`summary` stream claiming line 1 of `a.txt` is `foo\n` — while `a.txt` is a zero-byte file on disk: the panel must be empty behind its three blank gutter cells, with no source rows.

```bash
cd /home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough && ./demo-lines.sh
```

```output
ok: bom.txt row 1 shows hit right after the gutter -> yes
ok: bom.txt hit is the styled match at cells 0-2
ok: no visible BOM or fallback cell on the row -> no
ok: cr.txt row 1 shows the standalone CR as ^M -> yes
ok: cr.txt hit is styled after the ^M escape
ok: crlf.txt shows no ^M -> no
ok: crlf.txt row 1 is '1  hit' -> yes
ok: crlf.txt row 2 is '2  bye' -> yes
ok: crlf.txt hit is styled at cells 0-2
ok: runA exit status -> 0
ok: empty file: the claimed foo never displays -> no
ok: empty file: pane row 2 is blank -> 
ok: empty file: pane row 3 is blank -> 
ok: empty file: pane row 4 is blank -> 
ok: runB exit status -> 0
demo-lines: all checks passed
```

```bash
cd /home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough && echo "== bom.txt: first-line hit lands at cells 0-2 — no BOM bytes or ◌ cell painted ==" && sed -n "1,3p" lines/screen-01-bom.txt | sed "s/ *$//" && echo && echo "== cr.txt: the standalone CR renders as ^M inside the line ==" && sed -n "1,3p" lines/screen-02-cr.txt | sed "s/ *$//" && echo && echo "== crlf.txt: CRLF terminators paint nothing — no ^M ==" && sed -n "1,3p" lines/screen-03-crlf.txt | sed "s/ *$//" && echo && echo "== a.txt (zero bytes): empty panel, no source rows ==" && sed -n "1,4p" lines/screen-04-empty.txt | sed "s/ *$//" && echo && echo "== on-screen bytes (cat -v): no BOM bytes anywhere; ^M is literal caret-M text ==" && { sed -n "2p" lines/screen-01-bom.txt; sed -n "2p" lines/screen-02-cr.txt; sed -n "2,3p" lines/screen-03-crlf.txt; } | cat -v | sed "s/ *$//"
```

```output
== bom.txt: first-line hit lands at cells 0-2 — no BOM bytes or ◌ cell painted ==
./bom.txt   ── ./bom.txt ───────────────────────────────────────────────────────
./cr.txt    1  hit
./crlf.txt  2  rest

== cr.txt: the standalone CR renders as ^M inside the line ==
./bom.txt   ── ./cr.txt ────────────────────────────────────────────────────────
./cr.txt    1  a^Mb hit
./crlf.txt

== crlf.txt: CRLF terminators paint nothing — no ^M ==
./bom.txt   ── ./crlf.txt ──────────────────────────────────────────────────────
./cr.txt    1  hit
./crlf.txt  2  bye

== a.txt (zero bytes): empty panel, no source rows ==
./a.txt  ── ./a.txt ────────────────────────────────────────────────────────────




== on-screen bytes (cat -v): no BOM bytes anywhere; ^M is literal caret-M text ==
./cr.txt    1  hit
./cr.txt    1  a^Mb hit
./cr.txt    1  hit
./crlf.txt  2  bye
```

## Verdict

Issue #22 is verified. On the real binary with real rg, a CRLF file searches cleanly — no `^M` anywhere, `hit` highlighted at cells 0-2 — while a standalone CR mid-line renders as literal `^M` text and the match still lands on its own cells. The UTF-8 BOM file shows no BOM bytes and no `◌` fallback cell: the first-line match paints `hit` right after the gutter, proving rg's offset 0 was shifted three bytes into the raw view before validating and mapping. The fake-rg run proves the empty-file rule: a valid stream claiming line 1 of a zero-byte `a.txt` yields an empty panel — the claimed `foo` never displays, no source rows render, and the reserved gutter is the minimum one-digit slot plus two spaces (three blank cells, pinned directly by `TestEmptyFile` and `TestGutterWidth`). Underneath, `present.LineOfBOM` keeps the BOM in `Raw` while painting nothing, `filebuffer.Load` splits on LF with terminators retained, and every terminator byte and zero-width position maps to the display end-of-line position — the raw-file and rg-line coordinate views stay separate throughout.
