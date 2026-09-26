# Issue #30: Unsupported encodings — UTF-16/UTF-32 BOM placeholder

*2026-09-24T18:44:22Z by Showboat 0.6.1*
<!-- showboat-id: 635d1d05-3198-48ab-bf6d-cb626ffeb64a -->

Walkthrough for [Issue #30](../../../issues/030-unsupported-encodings-utf16-utf32.md), implementing the unsupported-encoding contract per `Notes/PRD-vrg.md` (*Encodings and stale-content validation* — the BOM bullet; *Invocation and child process* — ripgrep's default BOM detection stays enabled, no forced encoding flag; *Exit statuses* — unsupported files never change it): `filebuffer.Prepare` classifies the raw bytes through the longest-first `unsupportedBOMs` table — `FF FE 00 00` UTF-32 LE, `00 00 FE FF` UTF-32 BE, `FF FE` UTF-16 LE, `FE FF` UTF-16 BE — before any splitting or stale validation, while the UTF-8 BOM keeps its supported path. A classified buffer carries only its encoding name: the panel paints `(unsupported encoding)`, the filename row carries the matching status note, and the collected `cannot display <path>: unsupported encoding <name>` diagnostic follows Issue #26's current/non-current split — overlay when current, collection alone otherwise, re-opened from the retained lines on a later visit. The file stays an indexed cursor stop, `r` rereads it through the ordinary route, and the fixed exit status never moves. All generated artifacts live in this directory: the built `vrg` binary, the `demo-unsupported-encoding.sh` tmux harness, and its `encoding/` session captures. Test durations are stripped so the document verifies cleanly.

```bash
set -o pipefail; cd /home/chris/vrg && test -z "$(gofmt -l internal/ cmd/)" && echo GOFMT-CLEAN && go vet ./... && go build ./... && go test -count=1 ./... | sed "s/[[:space:]][0-9.]*s$//" && CGO_ENABLED=1 go test -race -count=1 ./internal/app ./internal/filebuffer | sed "s/[[:space:]][0-9.]*s$//" && go build -o Notes/walkthroughs/030-04/code-walkthrough/vrg ./cmd/vrg && echo GATES-OK
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
ok  	vrg/internal/filebuffer
GATES-OK
```

## BOM classification — `internal/filebuffer/encoding_test.go`

`Prepare` is both the first-load and the reload path, so the tests drive the classification in memory over raw bytes. `TestUnsupportedBOMsClassify` tables all four marks plus the bare UTF-16 mark: each names its encoding through `Unsupported()` while the buffer carries no lines, no spans, no stale verdict, and the inert `(0,0)` reveal target — the placeholder’s content-free state that keeps the file an indexed stop. `TestUTF32LEWinsOverlapWithUTF16LE` pins the load-bearing ordering: `FF FE 00 00` opens with UTF-16 LE’s own bytes, so the longer mark must be tried first. `TestLoadReportsUnsupported` covers the `ReadFile`-backed path; `TestUTF8BOMIsNotMisclassified` proves the leading UTF-8 BOM stays supported content that still validates; `TestNonBOMPrefixesStaySupported` keeps lone `FF`, non-leading `FF FE`, truncated `00 00 FE`, and plain text ordinary; `TestUnsupportedBytesSkipStaleValidation` shows recorded submatches that could never equal the encoded bytes marking nothing — Issue #29’s validator never runs on them.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestUnsupportedBOMsClassify|TestUTF32LEWinsOverlapWithUTF16LE|TestLoadReportsUnsupported|TestUTF8BOMIsNotMisclassified|TestNonBOMPrefixesStaySupported|TestUnsupportedBytesSkipStaleValidation" ./internal/filebuffer 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestUnsupportedBOMsClassify
    --- PASS: TestUnsupportedBOMsClassify/UTF-16_LE
    --- PASS: TestUnsupportedBOMsClassify/UTF-16_BE
    --- PASS: TestUnsupportedBOMsClassify/UTF-32_LE
    --- PASS: TestUnsupportedBOMsClassify/UTF-32_BE
    --- PASS: TestUnsupportedBOMsClassify/UTF-16_LE_bare_mark
--- PASS: TestUTF32LEWinsOverlapWithUTF16LE
--- PASS: TestLoadReportsUnsupported
--- PASS: TestUTF8BOMIsNotMisclassified
--- PASS: TestNonBOMPrefixesStaySupported
    --- PASS: TestNonBOMPrefixesStaySupported/single_FF
    --- PASS: TestNonBOMPrefixesStaySupported/FF_FE_not_at_start
    --- PASS: TestNonBOMPrefixesStaySupported/truncated_UTF-32_BE
    --- PASS: TestNonBOMPrefixesStaySupported/plain_text
--- PASS: TestUnsupportedBytesSkipStaleValidation
PASS
ok  	vrg/internal/filebuffer
```

## Placeholder, notification, and reload — `internal/app/encoding_test.go` + `outcome_test.go`

`TestUnsupportedCurrentFileNotifies` pins the current-file interrupt: the explanatory overlay opens over the `(unsupported encoding)` placeholder and the filename-row note, no encoded bytes and no inverse video paint, and one diagnostic is collected. `TestUnsupportedFileRemainsACursorStop` walks `n`/`p` through the file’s stops and proves a re-entry re-opens the retained overlay instead of the file-change pop-up. `TestUnsupportedNonCurrentIsDiagnosticOnly` is the other half of the Issue #26 split: a completion settling behind another current file collects once with no overlay and no repaint, and the visit surfaces it later. `TestUnsupportedReloadReissuesAndPreserves` covers the `r` route — swallowed by the open overlay, then `Loading…`, exactly one reread, and the still-encoded completion restoring the placeholder with a fresh overlay and a second collection. `TestUnsupportedFileIsNeverStale` and `TestUnsupportedComposedViewStaysWellFormed` close the stale-exclusion and composed-layout contracts at 80→20 columns. The `outcome_test.go` matrix gained the all-unsupported row: every retained file classified unsupported still exits with the fixed status 0.

```bash
cd /home/chris/vrg && go test -count=1 -v -run "TestUnsupportedCurrentFileNotifies|TestUnsupportedFileRemainsACursorStop|TestUnsupportedNonCurrentIsDiagnosticOnly|TestUnsupportedReloadReissuesAndPreserves|TestUnsupportedFileIsNeverStale|TestUnsupportedComposedViewStaysWellFormed|TestOutcomeMatrix/every_retained_file_unsupported" ./internal/app 2>&1 | grep -vE "^(=== RUN|=== CONT)" | sed -E "s/ \([0-9.]+s\)//; s/[[:space:]][0-9.]*s$//"
```

```output
--- PASS: TestUnsupportedCurrentFileNotifies
--- PASS: TestUnsupportedFileRemainsACursorStop
--- PASS: TestUnsupportedNonCurrentIsDiagnosticOnly
--- PASS: TestUnsupportedReloadReissuesAndPreserves
--- PASS: TestUnsupportedFileIsNeverStale
--- PASS: TestUnsupportedComposedViewStaysWellFormed
--- PASS: TestOutcomeMatrix
    --- PASS: TestOutcomeMatrix/every_retained_file_unsupported_keeps_the_fixed_status_0
PASS
ok  	vrg/internal/app
```

## Manual route — the real binary on a PTY

`demo-unsupported-encoding.sh` (checked into this directory) runs the freshly built `vrg` on a real tmux PTY with real `rg` over a disposable `mktemp` fixture — repository and user files are never touched, and an EXIT trap removes it. The fixture is `a.txt` plus `u16.txt` written with `printf '\xff\xfeh\0i\0\n\0'` — the UTF-16 LE mark followed by `hi\n` in two-byte code units, which `rg`’s default BOM detection still matches and transcodes while the raw file bytes stay encoded.

The session follows the issue’s manual route: `n` enters `u16.txt`, where the panel paints `(unsupported encoding)`, the filename row names the truncated-safe path with the same note, and the overlay carries `cannot display ./u16.txt: unsupported encoding UTF-16 LE` — no `^@` byte garbage anywhere. `r` while the overlay is open is the overlay’s, not a reload. After `Esc`, the file is swapped for a writerless FIFO so the `r` reread blocks on `open()`: `Loading…` is painted and held, and a duplicate `r` is dropped rather than queued (a queued press would re-block on the FIFO after settlement). A writer pulse of the same UTF-16 bytes settles the load: the placeholder returns with a fresh overlay — a second collected detection. `Esc` then `q` exits 0 and stderr replays both encoding diagnostics.

```bash
cd /home/chris/vrg/Notes/walkthroughs/030-04/code-walkthrough && unset RIPGREP_CONFIG_PATH && ./demo-unsupported-encoding.sh
```

```output
ok: startup: a.txt renders
ok: startup: filename rule names the path
ok: entering u16.txt: explanatory overlay
ok: entering u16.txt: panel placeholder
ok: entering u16.txt: filename row keeps the path
ok: entering u16.txt: no encoded bytes leak
ok: r under the overlay: no Loading…
ok: r under the overlay: overlay stays
ok: Esc: overlay dismissed
ok: Esc: placeholder stays
ok: r on fifo: Loading… while the read blocks
ok: r on fifo: filename row still names the path
ok: duplicate r mid-load: still just Loading…
ok: fifo settled: no queued second reread blocks
ok: fifo settled: placeholder restored
ok: fifo settled: fresh overlay
ok: vrg exit status -> 0
ok: stderr replays both detections -> 2
demo-unsupported-encoding: all checks passed
```

The deterministic model tests remain the authority for the classification table, the overlap ordering, the notification split, and the reload and stale-exclusion semantics; the PTY route demonstrates the visible behavior end to end — the placeholder, the filename-row note, the explanatory overlay, the held `Loading…` reload, and the exit-time stderr replay. Wiki ingest lives in `Notes/wiki/unsupported-encodings.md`.
