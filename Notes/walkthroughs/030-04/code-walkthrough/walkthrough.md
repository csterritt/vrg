# Issue #30: Unsupported encodings UTF-16/UTF-32

*2026-09-12T14:09:28Z by Showboat 0.6.1*
<!-- showboat-id: 67cceaf1-efb3-45ff-b182-eb3bbb56769f -->

Walkthrough for Issue #30 (Notes/tasks/030-unsupported-encodings-utf16-utf32.md), implementing UTF-16/UTF-32 BOM detection in the FileBuffer while preserving ripgrep's default encoding behavior. A detected UTF-16/UTF-32 file shows a safe (unsupported encoding) placeholder instead of attempting to render or transcode the file, retains its indexed cursor stops, remains reloadable through r, and follows the same current/non-current notification distinction as Issue #26 read failures. The stale-match guard (Issue #29) does not run against the raw encoded bytes. The fixed search-derived exit status is never altered merely because all matched files are unsupported. References: Notes/PRD-vrg.md (Encodings, Stale-content validation, Invocation), Notes/wiki/unsupported-encodings.md, Issue #30.

Contracts verified:

- All four unsupported BOMs are detected: UTF-16 LE (FF FE), UTF-16 BE (FE FF), UTF-32 LE (FF FE 00 00), UTF-32 BE (00 00 FE FF).
- Longer BOMs are checked before overlapping shorter ones so FF FE 00 00 classifies as UTF-32 LE, not UTF-16 LE.
- A leading UTF-8 BOM (EF BB BF) is not misclassified as unsupported.
- A detected file shows (unsupported encoding) with no file text and no highlights.
- An explanatory encoding diagnostic is collected for stderr replay.
- Unsupported files remain indexed cursor stops; n/p advance within the same file.
- The current/non-current notification distinction mirrors Issue #26: current opens the overlay and placeholder; non-current is diagnostic-only.
- r reloads the file (Loading... then re-detects the BOM, showing the placeholder with a fresh overlay).
- The stale-match guard (Issue #29) does not run against UTF-16/UTF-32 raw bytes; no file changed since search note.
- The all-unsupported outcome-matrix row keeps the fixed exit status unchanged (exit 0 for clean rg 0).
- The composed view is robust at ordinary and constrained widths: truncated safe path, no overflow, no negative dimensions.
- Ripgrep invocation is unchanged: no forced encoding flag; default BOM detection remains enabled.

The FileBuffer and App model tests are the authoritative deterministic verification. The manual route at the end demonstrates the behavior with a real UTF-16 LE file on disk.

```bash
go build ./cmd/... ./internal/... && go vet ./cmd/... ./internal/... && echo GATES-OK
```

```output
GATES-OK
```

```bash
go test -count=1 ./cmd/... ./internal/... -timeout 120s | sed 's/[[:space:]][0-9.]*s$//'
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

## FileBuffer BOM detection tests

The FileBuffer encoding tests (internal/filebuffer/encoding_test.go) verify that all four unsupported BOMs are detected; that longer BOMs are checked before overlapping shorter ones (FF FE 00 00 classifies as UTF-32 LE, not UTF-16 LE); that a leading UTF-8 BOM is not misclassified; that an unsupported buffer has no lines and no highlights; that it is not stale (the validation guard never runs); that reload preserves the placeholder; that the gutter width matches an empty file; and that short files and files without a BOM are not false positives.

```bash
go test -count=1 -v ./internal/filebuffer/ -run '^TestUnsupported|^TestUTF8BOM' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestUnsupportedUTF16LEBOM
--- PASS: TestUnsupportedUTF16LEBOM (0.00s)
=== RUN   TestUnsupportedUTF16BEBOM
--- PASS: TestUnsupportedUTF16BEBOM (0.00s)
=== RUN   TestUnsupportedUTF32LEBOM
--- PASS: TestUnsupportedUTF32LEBOM (0.00s)
=== RUN   TestUnsupportedUTF32BEBOM
--- PASS: TestUnsupportedUTF32BEBOM (0.00s)
=== RUN   TestUTF8BOMNotMisclassified
--- PASS: TestUTF8BOMNotMisclassified (0.00s)
=== RUN   TestUnsupportedNoHighlights
--- PASS: TestUnsupportedNoHighlights (0.00s)
=== RUN   TestUnsupportedNoStaleValidation
--- PASS: TestUnsupportedNoStaleValidation (0.00s)
=== RUN   TestUnsupportedReloadPreservesPlaceholder
--- PASS: TestUnsupportedReloadPreservesPlaceholder (0.00s)
=== RUN   TestUnsupportedGutterWidth
--- PASS: TestUnsupportedGutterWidth (0.00s)
=== RUN   TestUnsupportedNoBOMNotUnsupported
--- PASS: TestUnsupportedNoBOMNotUnsupported (0.00s)
=== RUN   TestUnsupportedShortFileNoFalsePositive
--- PASS: TestUnsupportedShortFileNoFalsePositive (0.00s)
=== RUN   TestUnsupportedUTF16LETwoByteOnly
--- PASS: TestUnsupportedUTF16LETwoByteOnly (0.00s)
PASS
ok  	vrg/internal/filebuffer
```

## App integration tests

The App encoding tests (internal/app/encoding_test.go) verify that a current-file unsupported encoding opens the non-fatal error overlay with the encoding diagnostic and shows (unsupported encoding) after dismissal while retaining cursor stops; that dismissing returns to browse with the placeholder still shown and q quits with the fixed exit status; that a non-current unsupported encoding is diagnostic-only (no overlay, no indicator) and is discovered by visiting the file; that r shows Loading... then re-detects the BOM with a fresh overlay; that the stale note never appears; that the all-unsupported outcome-matrix row keeps exit 0; that the composed view is robust at widths 80/40/20; and that a real UTF-16 LE file on disk is detected and presented through the App.

```bash
go test -count=1 -v ./internal/app/ -run '^TestUnsupported' -timeout 60s | sed 's/([0-9.]*s)/(0.00s)/g; s/[[:space:]][0-9.]*s$//'
```

```output
=== RUN   TestUnsupportedCurrentFileOverlayPlaceholder
--- PASS: TestUnsupportedCurrentFileOverlayPlaceholder (0.00s)
=== RUN   TestUnsupportedCurrentFileDismissReturnsToBrowse
--- PASS: TestUnsupportedCurrentFileDismissReturnsToBrowse (0.00s)
=== RUN   TestUnsupportedNonCurrentDiagnosticOnly
--- PASS: TestUnsupportedNonCurrentDiagnosticOnly (0.00s)
=== RUN   TestUnsupportedReloadViaR
--- PASS: TestUnsupportedReloadViaR (0.00s)
=== RUN   TestUnsupportedNoStaleNote
--- PASS: TestUnsupportedNoStaleNote (0.00s)
=== RUN   TestUnsupportedOutcomeAllUnsupportedFixed0
--- PASS: TestUnsupportedOutcomeAllUnsupportedFixed0 (0.00s)
=== RUN   TestUnsupportedComposedViewRobustness
=== RUN   TestUnsupportedComposedViewRobustness/width-80
=== RUN   TestUnsupportedComposedViewRobustness/width-40
=== RUN   TestUnsupportedComposedViewRobustness/width-20
--- PASS: TestUnsupportedComposedViewRobustness (0.00s)
    --- PASS: TestUnsupportedComposedViewRobustness/width-80 (0.00s)
    --- PASS: TestUnsupportedComposedViewRobustness/width-40 (0.00s)
    --- PASS: TestUnsupportedComposedViewRobustness/width-20 (0.00s)
=== RUN   TestUnsupportedRealFileUTF16LE
--- PASS: TestUnsupportedRealFileUTF16LE (0.00s)
PASS
ok  	vrg/internal/app
```

## Manual verification

The manual demo test (demo_artifacts/manual_demo_test.go) exercises the Issue #30 manual verification scenario with a real UTF-16 LE file on disk: create the file with printf '\xff\xfeh\0i\0\n\0', run vrg hi ., enter the file and show (unsupported encoding) with the overlay, dismiss the overlay, press r, show Loading..., show the unsupported placeholder with a new overlay, dismiss again, press q, verify exit status 0, and verify the encoding diagnostic appears on stderr.

```bash
cp Notes/walkthroughs/030-04/code-walkthrough/demo_artifacts/manual_demo_test.go internal/app/manual_demo_test.go && go test -count=1 -v -tags manual_demo ./internal/app/ -run '^TestManualDemoUnsupportedEncoding$' -timeout 30s 2>&1 | sed 's/[[:space:]]0\.[0-9]*s//g; s/([0-9.]*s)/(0.00s)/g'; rm internal/app/manual_demo_test.go
```

```output
=== RUN   TestManualDemoUnsupportedEncoding
    manual_demo_test.go:80: Step 2 OK: vrg hi . started, normal.txt loaded
    manual_demo_test.go:93: Step 3 OK: entered utf16.txt, overlay open with encoding diagnostic
    manual_demo_test.go:104: Step 4 OK: overlay dismissed, panel shows (unsupported encoding)
    manual_demo_test.go:114: Step 6 OK: panel shows Loading... during reload
    manual_demo_test.go:121: Step 7 OK: overlay open after reload
    manual_demo_test.go:132: Step 8 OK: overlay dismissed, panel shows (unsupported encoding)
    manual_demo_test.go:142: Step 10 OK: exit status 0
    manual_demo_test.go:156: Step 11 OK: UTF-16 diagnostic present in stderr replay: ["unsupported encoding: UTF-16 LE" "unsupported encoding: UTF-16 LE"]
    manual_demo_test.go:158: Manual demonstration passed: unsupported-encoding behavior matches the Issue #30 contracts.
--- PASS: TestManualDemoUnsupportedEncoding (0.00s)
PASS
ok  	vrg/internal/app
```
