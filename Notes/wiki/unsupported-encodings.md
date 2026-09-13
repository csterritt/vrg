# Unsupported encodings UTF-16/UTF-32 (Issue #30)

Issue #30 added detection of UTF-16 and UTF-32 BOMs to the FileBuffer
while preserving ripgrep's default encoding behavior. A detected
UTF-16/UTF-32 file shows a safe `(unsupported encoding)` placeholder
instead of attempting to render or transcode the file, retains its
indexed cursor stops, remains reloadable through `r`, and follows the
same current/non-current notification distinction as Issue #26 read
failures. The stale-match guard (Issue #29) does not run against the
raw encoded bytes. The fixed search-derived exit status is never
altered merely because all matched files are unsupported. Relevant PRD
section: *Encodings*. See
[read-failures-and-retry](read-failures-and-retry.md) for the
current/non-current notification pattern that Issue #30 mirrors,
[stale-match-validation](stale-match-validation.md) for the validation
exclusion, [explicit-reload](explicit-reload.md) for the `r` reload
that re-detects the BOM, and
[structural-line-handling](structural-line-handling.md) for the UTF-8
BOM handling that remains distinct from the unsupported-encoding BOMs.

## Problem

Before Issue #30, `filebuffer.Load` decoded every file as UTF-8 (after
optionally stripping a leading UTF-8 BOM). A UTF-16 or UTF-32 file was
treated as raw bytes: the BOM bytes became visible content, the
encoded text was misinterpreted as a sequence of bytes (many of them
NULs for UTF-16/32 BE), and the stale-match guard ran against these
raw encoded bytes, producing spurious "file changed since search"
notes. The panel rendered garbage and the user had no signal that the
encoding was the cause.

## Solution

Issue #30 added a `detectUnsupportedBOM` helper in `filebuffer`, two
metadata fields on `Buffer` (`UnsupportedEncoding` and
`EncodingDiagnostic`), an early-return path in `Load` that produces a
placeholder buffer with no lines or highlights, an
`unsupportedEncoding` flag on `app.Model`, integration with the Issue #9
error overlay, the Issue #24 filename-row status slot, the Issue #26
current/non-current notification split, the Issue #27 reload path, and
the Issue #29 stale-validation exclusion.

### BOM detection and overlap ordering

`detectUnsupportedBOM(data)` checks the leading bytes of the file
against the four unsupported-encoding BOMs:

| Encoding   | BOM bytes          |
|------------|--------------------|
| UTF-32 LE  | `FF FE 00 00`      |
| UTF-32 BE  | `00 00 FE FF`      |
| UTF-16 LE  | `FF FE`            |
| UTF-16 BE  | `FE FF`            |

The four-byte UTF-32 BOMs are checked **before** the two-byte UTF-16
BOMs because `FF FE` is both the UTF-16 LE BOM and the first two bytes
of the UTF-32 LE BOM (`FF FE 00 00`). Checking longer BOMs first
ensures `FF FE 00 00` classifies as UTF-32 LE, not UTF-16 LE.

A leading UTF-8 BOM (`EF BB BF`) is **not** an unsupported encoding.
`detectUnsupportedBOM` returns `""` for it, and the existing Issue #22
UTF-8 BOM stripping path handles it as before. The two detection
paths are mutually exclusive: `Load` checks for unsupported BOMs first
and returns early; only files without an unsupported BOM reach the
UTF-8 BOM strip.

### Placeholder buffer

When `detectUnsupportedBOM` returns a non-empty encoding name, `Load`
returns a `Buffer` with:

- `UnsupportedEncoding = true`
- `EncodingDiagnostic = "unsupported encoding: " + enc` (e.g.
  `"unsupported encoding: UTF-16 LE"`)
- `GutterWidth = gutterWidth(0)` (three cells, matching an empty file)
- No `Lines`, no `Highlights`, no `Stale` flag

The buffer carries no file text and no highlights. The stale-match
guard (Issue #29) never runs because `Load` returns before reaching
the validation code path. `Buffer.Stale` is therefore always `false`
for an unsupported-encoding file.

### App integration

`app.Model` carries an `unsupportedEncoding bool` flag, cleared on
reload (`r` shows `Loading…` then re-detects) and on cross-file
navigation, set when the current file's loaded buffer has
`UnsupportedEncoding == true`.

#### Current-file notification

When a `FileLoadCompleteMsg` for the current path carries a buffer
with `UnsupportedEncoding == true`:

1. `m.unsupportedEncoding` is set true.
2. The Issue #9 error overlay opens with the encoding diagnostic
   (non-fatal: dismissal returns to browse, `q` still quits).
3. The content panel shows `(unsupported encoding)` instead of file
   text or `Loading…`.
4. The filename-row status slot (Issue #24) shows
   `(unsupported encoding)`.
5. The encoding diagnostic is collected for stderr replay (Issue #11).
6. No viewport is built (no file text), so no layout preparation is
   requested.

The file's cursor stops are retained: `n`/`p` advance within the same
file as usual. The filename row still identifies the path (subject to
the same truncation and constrained-width rules as other status
notes).

#### Non-current-file notification

When a `FileLoadCompleteMsg` for a non-current path carries a buffer
with `UnsupportedEncoding == true`:

1. The buffer is cached (Issue #13 session cache).
2. The encoding diagnostic is collected for stderr replay (Issue #11).
3. No overlay opens, no indicator appears, and the visible panel is
   untouched.

The non-current unsupported file is discovered by navigating to it:
the cached buffer's `UnsupportedEncoding` flag opens the overlay and
shows the placeholder through the cached-destination path in
`handleNavigate`.

#### Reloadability through `r`

Pressing `r` on an unsupported-encoding file follows the Issue #27
reload path:

1. `handleReload` clears `unsupportedEncoding` and `readFailed`, sets
   `loading = true`, and issues a reread via `startLoad`.
2. The panel shows `Loading…` (not `(unsupported encoding)`) while
   the reload is in flight.
3. The completion re-runs `filebuffer.Load`, which re-detects the BOM.
4. If the file is unchanged, the placeholder returns with a fresh
   overlay.
5. If the file has been converted to UTF-8 (BOM removed), the normal
   decode path runs and the file text is shown.

The reload does not rerun ripgrep or change cursor stops. The content
revision is incremented (Issue #27), invalidating any cached layout
from the prior revision — though unsupported-encoding buffers have no
viewport, so this is a no-op in practice.

### Stale-validation exclusion

The Issue #29 stale-match guard validates submatches against the
original line bytes. For a UTF-16/UTF-32 file, those bytes are raw
encoded data that would never match the UTF-8 match bytes ripgrep
recorded, producing a spurious "file changed since search" note.
Issue #30 excludes these files by returning the placeholder buffer
before the validation code path in `Load`. `Buffer.Stale` is always
`false` for an unsupported-encoding file, and the filename-row note
never appears.

### Fixed exit-status guarantee

Unsupported encodings affect only file presentation and diagnostics,
not the exit status. `DecideOutcome` computes the exit status solely
from the process exit, stream integrity, usable results,
diagnostics, and record loss. An all-unsupported index (every matched
file has an unsupported encoding) with rg 0, a clean complete stream,
and results still exits 0. The all-unsupported outcome-matrix row
(`TestUnsupportedOutcomeAllUnsupportedFixed0`) proves this.

### Ripgrep invocation unchanged

Issue #30 does not add a forced raw-encoding flag to the child
ripgrep argv. Ripgrep's default BOM detection remains enabled. The
PRD contract (*Invocation*) is unchanged: vrg forwards the user's
flags and pattern, and ripgrep decides how to search each file. vrg
only decides how to *present* a file whose BOM it detects as
unsupported.

### Composed-view robustness

The `(unsupported encoding)` placeholder is truncated to the panel
width via `truncateRightCells` so the composed view never overflows
at constrained widths. The filename row truncates the safe path per
the Issue #24 slot rules. Layout dimensions stay nonnegative. The
`TestUnsupportedComposedViewRobustness` test verifies this at widths
80, 40, and 20.

## Test coverage

`internal/filebuffer/encoding_test.go` covers the FileBuffer-level
contracts: detection of all four BOMs (UTF-16 LE/BE, UTF-32 LE/BE),
overlap ordering (UTF-32 LE `FF FE 00 00` classifies as UTF-32 LE, not
UTF-16 LE), UTF-8 BOM non-misclassification, absence of highlights,
absence of stale validation, reload preserving the placeholder,
gutter width matching an empty file, and false-positive avoidance
(short files, no BOM).

`internal/app/encoding_test.go` covers the App-level contracts:
current-file overlay and placeholder, dismiss returning to browse,
non-current diagnostic-only behavior, `r` reload showing `Loading…`
then re-detecting with a fresh overlay, absence of the stale note,
all-unsupported outcome-matrix row keeping exit 0, composed-view
robustness at widths 80/40/20, and a real UTF-16 LE file on disk
detected through `filebuffer.Load` and presented through the App.
