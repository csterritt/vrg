# Unsupported encodings — UTF-16/UTF-32 BOM placeholder

Issue #30
(`Notes/issues/030-unsupported-encodings-utf16-utf32.md`,
tasks
`Notes/tasks/030-unsupported-encodings-utf16-utf32.md`)
added the unsupported-encoding contract: a file whose bytes open with
a UTF-16 or UTF-32 byte-order mark is detected at load, presented as
the `(unsupported encoding)` placeholder instead of escaped garbage,
explained by a collected diagnostic under the Issue #26
current/non-current notification split, and kept an ordinary indexed
stop — reloadable by `r`, never stale-validated, and never moving the
fixed search-derived exit status.

PRD cross-references: "Encodings and stale-content validation" (the
first bullet — BOM detection, the placeholder, and the exclusion of
encoded bytes from stale validation); "Invocation and child process"
(ripgrep's default BOM detection stays enabled — vrg forces no
encoding flag on the child argv, so rg transcodes what it reports
while the raw file bytes stay encoded); "Exit statuses" ("unreadable,
stale, or unsupported files do not change it"); and "Testing
Decisions → FileBuffer" (the UTF-16/32 placeholder fixtures) in
`Notes/PRD-vrg.md`.

## BOM detection

`filebuffer.Prepare` classifies the raw bytes before any splitting or
validation through `unsupportedEncoding`, the longest-first ordered
table `unsupportedBOMs`:

| Mark          | Encoding   |
| ------------- | ---------- |
| `FF FE 00 00` | UTF-32 LE  |
| `00 00 FE FF` | UTF-32 BE  |
| `FF FE`       | UTF-16 LE  |
| `FE FF`       | UTF-16 BE  |

The ordering is load-bearing: UTF-32 LE's `FF FE 00 00` opens with
UTF-16 LE's own bytes, so trying UTF-16 first would misclassify it —
the overlap case the tests pin. UTF-32 BE's `00 00 FE FF` cannot
collide with UTF-16 BE's leading `FE FF`, but the uniform
longest-first table keeps the rule obvious. The leading UTF-8 BOM is
**not** an unsupported mark: it takes the Issue #22 path — invisible
three bytes, first-line coordinate shift — and still validates. Marks
not at the file's very start are ordinary content: a BOM must lead.

A classified buffer carries `enc` — the encoding's display name —
with no lines, no spans, and no stale verdict: the encoded bytes never
reach `present.LineOf`, so no `^@`/U+FFFD garbage and no inverse
highlight can paint. `Buffer.Unsupported()` reports the name; `""`
means ordinary displayable content. `RevealTarget` on the zero-line
buffer returns the inert `(0,0)` — the file stays a cursor stop
without an invented landing.

## Presentation and notification

The placeholder is `contentRow`'s third form: behind the minimal
one-digit gutter, `(unsupported encoding)` clipped to the text width
— no file text, no highlights. `bufferNote` adds
`(unsupported encoding)` to the Issue #24 filename-row status slot,
ranked below `(unreadable)` and `file changed since search` — ranks
that can never co-fire, since an unsupported buffer is never failed
and never stale.

Detection produces the diagnostic `cannot display <path>: unsupported
encoding <name>`, sanitized through `present.Diagnostic` and collected
once per detection into the session collection (`diags`) for exit-time
replay. The Issue #26 split decides notification: when the path is
current the explanatory overlay opens (appending a new occurrence when
one is already up); when it is not, the collection alone takes it —
no overlay, no in-UI indicator, no repaint of the current panel.

The retained per-path `encLines` map holds the detection's sanitized
lines so a later visit re-opens the same overlay — the Issue #26
re-entry pattern applied to encodings, replacing the file-change
pop-up on that crossing. A successful load with no unsupported mark
clears `encLines`; a read failure clears it too, the failure's own
state taking over.

## Reloadability and stale exclusion

The unsupported file is an ordinary reloadable file: `r` drops the
cached buffer, the panel reads `Loading…` while exactly one reread is
in flight (duplicates dropped, not queued), and a still-encoded
completion restores the placeholder with a fresh overlay occurrence
and a second collected diagnostic. The open overlay owns `r` while it
is up — dismissal first, then the key reaches the reload route.
Entering an unsupported file never re-reads it: the buffer is cached,
`ensureLoad` drops the request, and only the overlay re-opens.

Encoded bytes never face Issue #29's stale-match validation:
`Prepare` returns before the validation loop, so rg's transcoded
submatch offsets — which can never equal the raw UTF-16/32 bytes —
cannot mark the file stale or produce the `file changed since search`
note. This is the PRD's explicit exclusion applied at the single point
all loads funnel through.

## Fixed exit status

Unsupported classification is presentation-only. The outcome decision
ran at `searchDoneMsg` before any load, and detection marks no exit
code: an index whose every retained file is unsupported still exits
with the fixed status — 0 for a clean usable-results search. The
outcome matrix carries the all-unsupported row proving it.

## Tests

- `internal/filebuffer/encoding_test.go` (Issue #30) drives `Prepare`
  and `Load` over raw marked bytes: `TestUnsupportedBOMsClassify`
  (all four marks plus the bare UTF-16 mark → named encoding, no
  lines, no spans, no stale, inert reveal target);
  `TestUTF32LEWinsOverlapWithUTF16LE` (the overlap ordering);
  `TestLoadReportsUnsupported` (the `ReadFile` path);
  `TestUTF8BOMIsNotMisclassified` (the UTF-8 BOM stays supported and
  validating); `TestNonBOMPrefixesStaySupported` (lone `FF`,
  non-leading `FF FE`, truncated `00 00 FE`, plain text);
  `TestUnsupportedBytesSkipStaleValidation` (mismatching and
  out-of-range recorded submatches mark nothing).
- `internal/app/encoding_test.go` (Issue #30) pins the integration:
  `TestUnsupportedCurrentFileNotifies` (overlay + placeholder +
  filename-row note + one collection, no file text or inverse video);
  `TestUnsupportedFileRemainsACursorStop` (`n`/`p` traversal of the
  file's stops and the re-entry overlay on return);
  `TestUnsupportedNonCurrentIsDiagnosticOnly` (collection without
  overlay or repaint, visit surfacing the retained lines);
  `TestUnsupportedReloadReissuesAndPreserves` (`r` swallowed by the
  open overlay, `Loading…`, exactly one reread, placeholder plus
  fresh overlay on the still-encoded completion);
  `TestUnsupportedFileIsNeverStale` (no stale mark, no note);
  `TestUnsupportedComposedViewStaysWellFormed` (long escaped path at
  80→20 columns: truncated safe path, visible placeholder, no
  overflow, nonnegative layout).
- `internal/app/outcome_test.go` gained the Issue #30 row on the
  existing `fileData`/`loadCurrent` fields: every retained file
  detecting an unsupported encoding still exits with the fixed
  status 0, the overlay and the exit replay carrying the diagnostic.

See [unit-tests.md](unit-tests.md) § `internal/filebuffer` and
§ `internal/app`.

## Files

- `internal/filebuffer/filebuffer.go` — `unsupportedBOMs`,
  `unsupportedEncoding`, the `Prepare` early return, `Buffer.enc`,
  `Unsupported()`.
- `internal/app/app.go` — the `encLines` per-path map and the
  `loadDoneMsg` success branch's detect-collect-retain-overlay split.
- `internal/app/browse.go` — `contentRow`'s `(unsupported encoding)`
  placeholder, `bufferNote`'s third note, and `navigate`'s
  retained-lines overlay route.

See also:
[line-terminators-and-bom.md](line-terminators-and-bom.md) (the UTF-8
BOM the detection must not misclassify, and the raw-byte view),
[stale-match-validation.md](stale-match-validation.md) (the validation
encoded bytes are excluded from),
[read-failures.md](read-failures.md) (the current/non-current
notification split the diagnostic follows),
[explicit-reload.md](explicit-reload.md) (the `r` reread the
placeholder survives),
[async-load-isolation.md](async-load-isolation.md) (the keyed
completion the detection rides),
[match-navigation.md](match-navigation.md) (the stops the file
remains),
[error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
(the overlay and the fixed-status matrix row),
[file-list-layout.md](file-list-layout.md) (the status-note slot the
encoding note occupies), and
[stderr-replay.md](stderr-replay.md) (the collection and exit replay
the diagnostic joins).
