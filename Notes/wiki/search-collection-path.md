# Search collection path (Issue #3, extended by Issues #4, #8, #9, #10, and #11)

The ripgrep execution and result-collection pipeline delivered by
[Issue #3](../issues/003-spawn-rg-collect-results-searching-screen.md),
replacing the Issue #2 stub with real subprocess execution, JSON
stream collection, and an interim searching/summary TUI.
[Issue #4](../issues/004-cancellation-child-cleanup-terminal-restore.md)
added cancellation, child termination/reaping, terminal restoration,
and controlled-failure cleanup. [Issue #8](../issues/008-no-results-screen-and-binary-exclusion.md)
added binary-file exclusion during indexing and a distinct no-results
TUI outcome for searches that complete successfully but yield no usable
results. [Issue #9](../issues/009-error-overlay-and-fatal-outcomes.md)
added stream-integrity accounting, separate process-success and
stream-integrity assessment, the fatal/warning outcome matrix, and the
modal error overlay. [Issue #10](../issues/010-record-robustness-malformed-oversized-unknown.md)
added robust handling of malformed, oversized, and unknown-type records
with separate counters, bounded 64 MiB record parsing with
discard-and-resynchronize behavior, sanitized oversized-record
diagnostics, and record-loss outcome rows. [Issue #11](../issues/011-stderr-replay-of-collected-diagnostics.md)
added the session diagnostic collection independent of display, the
processed-versus-in-flight shutdown boundary, and post-restoration
stderr replay of every collected diagnostic exactly once in collection
order. Relevant PRD sections:
*Implementation Decisions → Invocation and child arguments*,
*Module Design → CLI / SearchIndex / App*, *Testing Decisions → CLI /
SearchIndex / App / Subprocess boundary / Responsiveness boundaries*,
*Outcome and exit-status contract*, *Colours, overlays, and key
precedence* (replay bullet), and *Resources and responsiveness
(64 MiB record limit)*. See also [outcome-contract](outcome-contract.md),
[record-robustness](record-robustness.md), and
[safe-presentation](safe-presentation.md).

## Process boundary

`cmd/vrg/main.go` is the sole process boundary. `runSearch`:

1. Resolves the working directory with `os.Getwd()`.
2. Constructs `exec.Command("rg", res.ChildArgs...)` using the protected
   child argv from the [CLI contract](cli-flag-forwarding.md) and sets
   `rgCmd.Dir` to the working directory. Since Issue #4, the child is
   placed in its own process group (`Setpgid: true`) so cleanup can
   kill the entire group.
3. Creates stdout and stderr pipes via `StdoutPipe`/`StderrPipe`.
4. Starts ripgrep with `rgCmd.Start()`. A start failure (rg not on PATH,
   exec error) prints a sanitized diagnostic to stderr and returns exit
   2 without entering the TUI.
5. Constructs an `app.Process` via `app.NewProcess` (Issue #4) with the
   running process and its pipes, then constructs an `app.Model` with
   `app.WithProcess` and optional test seams.
6. Runs the Bubble Tea program with `tea.NewProgram(model,
   tea.WithOutput(stdout))`.
7. After the program exits, performs centralized cleanup (Issue #4):
   kills the child process group with `syscall.Kill(-pid, SIGKILL)`,
   then calls `proc.Cleanup()` which kills the direct child if still
   running and waits for the collection goroutine to finish (ensuring
   the child is reaped).
8. Inspects the final model state: if a diagnostic is present, prints
   it to stderr exactly once, after the terminal has been restored by
   Bubble Tea. Since Issue #11, replays every collected diagnostic from
   `m.Diagnostics()` to stderr, exactly once each, in collection order,
   after terminal restoration. The controlled-failure diagnostic is
   routed through the collection (no separate direct write), so the
   Issue #4 post-restoration writer serves every controlled exit.
9. Returns the model's exit code (0 for normal quit, 130 for
   Ctrl-C/quit during search, 2 for failure).

The library packages never call `os.Exit` or write directly to
stdout/stderr. All exit-status and stream-destination decisions live in
`cmd/vrg`.

## Ripgrep JSON schema

The `internal/searchindex` package parses ripgrep's `--json` stream.
Recognized event types:

- `begin` — opens a file; carries the path.
- `match` — a matched line; carries path, line number, line text/bytes,
  and submatches.
- `end` — closes a file. Carries `binary_offset`: a null value means the
  file is text (Issue #8 retains its matches); a non-null nonnegative
  integer means ripgrep detected binary content, and Issue #8 drops the
  file and all its previously collected matches.
- `summary` — final stream record (currently carries no data we index).
- `context` — recognized but ignored (Issue #3 does not display context
  lines).

Both `text` and `bytes` encodings are supported for paths and line
content. Text and bytes representations of the same logical value produce
identical index entries. Raw non-UTF-8 bytes are retained when ripgrep
reports the `bytes` encoding, so path identity and line content are
preserved exactly.

### Binary exclusion (Issue #8)

A valid `end` event with a non-null `binary_offset` excludes its file:

- All previously collected matches for that file are dropped from the
  builder.
- The file is counted as a distinct excluded file.
- Match records arriving after the binary `end` for the same file are
  also dropped.
- Matches for other files are unaffected.

A missing `end` event does not confirm nonbinary status; existing
matches are retained. The usable-results value is the retained stop
count after binary filtering, never the raw received match-event
count. This is the single value the app outcome logic consumes.

## SearchIndex

`internal/searchindex/searchindex.go` provides:

- `Index` — the prepared, navigable result index.
- `Stop` — a single matched line (file, line number, line bytes,
  submatches).
- `Submatch` — one match within a line (text/bytes and a `Range`).
- `Range` — a half-open `[start, end)` byte range.
- `Builder` — accumulates parsed events and produces an `Index` via
  `Build()`.

### Parsing and merging

`Builder.Add` decodes one JSON record. `begin` opens a file context.
`match` records a stop, merging same-line matches: if a new match shares
the same file and line number as an existing stop, its submatches are
appended and the line bytes are kept from the first occurrence. `end`
closes the file context. `summary` and `context` are recognized but do
not affect the index.

### Range normalization

Submatch ranges are sorted by start offset. Overlapping or adjacent
ranges are merged: `[0,5)` and `[3,8)` become `[0,8)`; `[0,5)` and
`[5,10)` (adjacent) become `[0,10)`. Contained ranges are absorbed.

### Path handling

Paths are resolved relative to the working directory without
canonicalization. Path identity for same-line merging is based on raw
path bytes, not the resolved string, so two paths that resolve to the
same file but differ in byte representation are treated as distinct
during parsing. Navigation ordering uses unsigned byte ordering on the
raw path bytes, so non-UTF-8 paths sort after ASCII paths by byte value.

### API

- `Index.Len()` — number of stops (matched lines).
- `Index.Stops()` — a copy of the stops slice (caller-safe).
- `Index.Files()` — number of distinct files with retained matches.
- `Index.ExcludedFiles()` (Issue #8) — number of distinct files dropped
  by a non-null `binary_offset` in their `end` event.
- `Index.Integrity()` (Issue #9) — `Integrity{Complete}`, true only when
  every lifecycle rule passed.
- `Index.MalformedCount()` (Issue #10) — number of records skipped and
  counted as malformed (invalid JSON, invalid base64, missing/invalid
  type, or known events violating the per-record schema matrix). Kept
  separate from integrity failures except where the matrices mark both.
- `Index.OversizedCount()` (Issue #10) — number of records that exceeded
  the 64 MiB payload limit and were discarded.
- `Index.UnknownCount()` (Issue #10) — number of records with an
  unrecognised string event type, counted separately from malformed.
- `Index.OversizedDiagnostics()` (Issue #10) — per-record oversized
  diagnostics with sanitized paths, for records where path recovery
  succeeded. Each entry is of the form "oversized record skipped for
  <sanitized path>".

### Bounded record parsing (Issue #10)

`Builder.ReadFrom(io.Reader) (int64, error)` reads newline-delimited
JSON records with an explicit bounded reader rather than a small
default line-reader limit. The 64 MiB payload limit (`MaxRecordSize`)
excludes the newline delimiter:

- A record exactly at the 64 MiB limit is accepted.
- A record one byte over is discarded as oversized.
- An oversized record is consumed and discarded through its next
  newline, so parsing resynchronizes on the following record. Oversized
  records never permanently desynchronize the stream.
- A trailing oversized record without a newline is counted oversized
  **and also counted malformed** for its missing termination, and marks
  the stream incomplete — all three dispositions asserted (oversized
  count, malformed count, incomplete integrity), matching the PRD's
  unqualified trailing-unterminated rule.
- An ordinary trailing record without a newline is counted malformed
  and marks the stream incomplete (the Issue #9 rule, unchanged).

`collectResults` (in `internal/app`) now uses `Builder.ReadFrom` instead
of `bufio.Scanner`. A `ReadFrom` I/O error marks the stream as having a
trailing malformed record so the outcome is fatal.

## App model

`internal/app/app.go` implements the Bubble Tea model:

- `Model` — the app state, carrying child argv, working directory,
  process handle, gate channel, failure signal channel, file/line
  counts, diagnostic, exit code, and a cancelled flag.
- `New(childArgs, workdir, opts...)` — constructs a model in the
  searching state.
- `NewProcess(cmd, stdout, stderr)` (Issue #4) — creates a `*Process`
  with lifecycle channels (`done`, `cancel`) initialized.
- `WithProcess(p)` — injects a running ripgrep process.
- `WithGate(ch)` — test seam: holds index preparation until the channel
  is closed or receives, so tests can verify the searching state persists
  after rg exits but before the index is ready.
- `WithFailureSignal(ch)` (Issue #4) — test seam: a channel whose
  receipt triggers a `ControlledFailureMsg` with the received string as
  the diagnostic.
- `WithDiagnosticSignal(ch)` (Issue #11) — test seam: a channel whose
  receipt emits a `DiagnosticMsg` for collection into the session
  diagnostic collection without display. In the same mechanism family
  as `WithFailureSignal`.
- `WithOnCollect(f)` (Issue #11) — test seam: a callback called once a
  diagnostic has been processed into the session collection. The
  application-side acknowledgement side channel, in the same mechanism
  family as `Process.OnReap`.
- `State()` / `ExitCode()` / `Diagnostic()` — accessors for the entry
  point.
- `Diagnostics()` (Issue #11) — returns a copy of the session
  diagnostic collection in collection order. Each entry is sanitized
  through `sanitizeDiagnostic`. The entry point replays these to
  stderr after terminal restoration, exactly once each, on every
  controlled exit. The collection is independent of what was displayed.

### States

- `StateSearching` — initial state while rg is running and results are
  being collected.
- `StateSummary` — collection and index preparation are complete; the
  interim summary is shown (backward-compatible path when no index is
  available).
- `StateBrowse` — two-pane browse state shown after a completed search
  with usable results.
- `StateNoResults` (Issue #8) — centred no-results screen shown after a
  complete successful search (rg exit 0 or 1) with no usable results.
  The optional `(N binary files skipped)` suffix is appended when every
  matched file was excluded.
- `StateStartFailed` — rg could not be started; the entry point should
  print the diagnostic and exit 2.
- `StateFailed` (Issue #4) — controlled application failure after the
  child started; the entry point prints the diagnostic and exits 2.
- `StateCancelled` (Issue #4) — cancellation via `q` while searching or
  `ctrl+c` in any state. Late search completions are ignored.

### Lifecycle

`Init()` returns a `tea.Batch` of `collectResults` and (if a failure
signal was injected) `watchFailure`, and (if a diagnostic signal was
injected, Issue #11) `watchDiagnostic`. `collectResults` runs as a Bubble
Tea command:

1. Drains stderr concurrently in a goroutine (`io.Copy` into a buffer) so
   a large stderr stream cannot block ripgrep while stdout collection
   waits.
2. Parses stdout records into a `searchindex.Builder` via
   `Builder.ReadFrom` (Issue #10), the bounded 64 MiB record reader that
   discards oversized records through the next newline and resynchronizes
   on the following record. A `ReadFrom` I/O error marks the stream as
   having a trailing malformed record so the outcome is fatal.
3. Waits for the stderr drain goroutine to finish.
4. Waits for ripgrep to exit with `p.Cmd.Wait()` (reaping the child).
   If `OnReap` is set, calls it with the wait error (test seam for
   proving the reap path ran).
5. Holds at the gate if set (test seam). The gate select is
   cancellable: if the cancel channel is closed (cancellation), the
   goroutine skips index preparation and returns promptly.
6. Builds the index and returns a `SearchCompleteMsg` with file and
   line counts.
7. Closes the `done` channel on exit (deferred), so the process
   boundary can wait for the goroutine to finish.

`watchFailure` (Issue #4) waits on the failure signal channel and emits
a `ControlledFailureMsg` when it fires.

`watchDiagnostic` (Issue #11) waits on the diagnostic signal channel and
emits a `DiagnosticMsg` when it fires. The diagnostic is collected into
the session collection without display; the entry point replays it to
stderr after terminal restoration.

`Update` handles:

- `SearchCompleteMsg` — if not cancelled, transitions based on the
  index: with usable results (Len > 0) to `StateBrowse` and loads the
  first file; with no usable results and a non-nil index (Issue #8) to
  `StateNoResults`, recording the excluded-file count for the binary
  skip suffix; with a nil index (backward-compatible path) to
  `StateSummary`. If cancelled, ignores the message (late-completion
  rejection). Since Issue #11, the overlay text from the outcome
  decision is collected into the session diagnostic collection
  regardless of whether the overlay is later displayed or dismissed.
- `SearchFailedMsg` — transitions to `StateStartFailed`, records the
  sanitized diagnostic, sets exit code 2, collects the diagnostic into
  the session collection (Issue #11), and quits.
- `ControlledFailureMsg` (Issue #4) — transitions to `StateFailed`,
  records the sanitized diagnostic, sets exit code 2, cancels the
  collection goroutine, collects the diagnostic into the session
  collection (Issue #11, routing it through the collection instead of a
  separate direct write), and quits.
- `DiagnosticMsg` (Issue #11) — collects the diagnostic into the
  session collection without displaying it in an overlay. This path
  serves diagnostics that are only recoverable through stderr replay.
- `tea.KeyPressMsg` — `q` while searching cancels (exit 130); `q` from
  summary quits (exit 0); `q` from no-results (Issue #8) quits with exit
  1 through the same cleanup path as browse; `q` from browse quits
  (exit 0); Ctrl-C in any state cancels (exit 130); escape is a no-op.
- `tea.WindowSizeMsg` — records width and height.

`View` renders:

- `StateSearching` — `Searching…` (alt screen enabled).
- `StateSummary` — `N files, M matched lines` (alt screen enabled).
- `StateNoResults` (Issue #8) — centred `No results found`, with the
  optional `(N binary files skipped)` suffix appended when every matched
  file was excluded and `N > 0` (alt screen enabled).
- `StateBrowse` — two-pane browse view (alt screen enabled).
- other states — empty.

### Cancellation (Issue #4)

`q` while searching (including the post-rg-exit/preparation window when
the gate holds index preparation) and `ctrl+c` in any state cancel
outstanding work:

1. The model transitions to `StateCancelled`, sets `cancelled = true`,
   sets exit code 130, and calls `process.Cancel()` (closes the cancel
   channel, releasing the collection goroutine from the gate).
2. The model returns `tea.Quit`.
3. The process boundary kills the child process group and calls
   `proc.Cleanup()`, which waits for the collection goroutine to finish.
4. The collection goroutine, unblocked by the child's termination
   (pipes close), calls `Wait()` (reaping the child), and closes `done`.
5. Late `SearchCompleteMsg` from the goroutine is ignored by the
   cancelled model.

### Controlled failure (Issue #4)

An injectable failure hook (`WithFailureSignal`) lets tests trigger a
controlled application failure after the child has started. The failure
path:

1. The model transitions to `StateFailed`, records the sanitized
   diagnostic, sets exit code 2, cancels the collection goroutine, and
   quits.
2. The process boundary kills the child process group and calls
   `proc.Cleanup()`.
3. After the terminal is restored by Bubble Tea (on `tea.Quit`), the
   process boundary writes the diagnostic to stderr exactly once
   through the single post-restoration stderr writer.
4. The diagnostic is never written both directly and through a later
   replay mechanism (exactly-once across mechanisms).
5. Exit status is 2.

Since Issue #11, the controlled-failure diagnostic is routed through
the session diagnostic collection (via `collectDiagnostic`) instead of
a separate direct write, so the Issue #4 post-restoration writer serves
every controlled exit. Exactly-once holds across both the former
direct-write path and the replay mechanism.

### Session diagnostic collection and replay (Issue #11)

The model maintains a session diagnostic collection (`diagnostics
[]string`) independent of what was displayed. Every diagnostic the
model processes is collected via `collectDiagnostic`, which sanitizes
the text through `sanitizeDiagnostic`, appends it to the collection,
and fires the `onCollect` callback if set.

Collection sources:

- `SearchCompleteMsg` — the overlay text from the outcome decision is
  collected regardless of whether the overlay is later displayed or
  dismissed.
- `SearchFailedMsg` — the start-failure diagnostic is collected.
- `ControlledFailureMsg` — the controlled-failure diagnostic is
  collected (routing it through the collection instead of a separate
  direct write).
- `DiagnosticMsg` — diagnostics collected without display, for
  diagnostics only recoverable through stderr replay.

The shutdown boundary is defined at message-processing time: a
diagnostic is "collected" once the model has processed the message
carrying it. A diagnostic still in flight (e.g., a gated
`SearchCompleteMsg`) has not been processed, is not collected, is not
waited for, and is not replayed. This applies to both cancellation
keys: `ctrl+c` in any state and `q` while searching or gate-held
preparation is incomplete.

After `program.Run()` returns and cleanup is complete, the process
boundary replays every collected diagnostic to stderr, exactly once
each, in collection order. Replay occurs strictly after the
display-restoration sequence (alt-screen exit, cursor show) and after
input modes are restored, because `program.Run()` returns only after
Bubble Tea restores the terminal. Replay does not wait on unrelated
in-flight work: only diagnostics already processed by the model before
the exit are collected.

The `onCollect` callback is the application-side acknowledgement side
channel, in the same mechanism family as `Process.OnReap`. The process
boundary wires it via an environment-variable-gated side channel
(`VRG_TEST_COLLECT_ACK`) so PTY tests can wait for the acknowledgement
before sending the exit key, proving the model has processed the
diagnostic into the session collection rather than merely that bytes
reached the pipe.

### No-results outcome (Issue #8)

A complete successful search with no usable results presents the
no-results screen. This covers two cases:

- ripgrep exit status 1 with an empty stream (no matches at all).
- ripgrep exit status 0 where every matched file was binary-excluded.

The screen shows centred `No results found`. When every matched file
was excluded, the suffix `(N binary files skipped)` is appended, where
`N` is the distinct excluded-file count from the index. The
usable-results value is the retained stop count after binary
filtering, exposed as the single value the outcome logic consumes.

Key behavior from the no-results screen:

- `q` dismisses the screen and exits 1 through the same Issue #4
  cleanup path as browse (process cancellation, child reaping,
  terminal restoration).
- `Esc` is a no-op.
- `ctrl+c` exits 130 through the existing cancellation/cleanup paths.

Mixed streams (one file excluded, one retained) transition to ordinary
browsing with the retained stops; the no-results screen is not shown.

### Terminal restoration (Issue #4)

Bubble Tea's renderer enables the alternate screen and hides the
cursor on startup. On `tea.Quit`, the renderer exits the alt screen
(`\x1b[?1049l`) and shows the cursor (`\x1b[?25h`), restoring both the
display and the PTY input modes (raw/no-echo undone). The process
boundary writes any diagnostic after `program.Run()` returns, ensuring
it appears after terminal restoration.

### Sanitization

`sanitizeDiagnostic` strips ANSI escape bytes (0x1b, 0x9b) from
diagnostic strings so they are safe to write to stderr. The entry point
applies `cli.Escape` to start-failure errors before printing them.

## Dual-pipe drainage

The concurrent stdout/stderr drainage is critical: ripgrep writes
JSON records to stdout and diagnostics to stderr. If only stdout is
drained, a stderr stream exceeding the pipe buffer (typically 64 KiB)
deadlocks ripgrep, which blocks on stderr write while vrg waits on
stdout. `collectResults` drains both pipes in separate goroutines and
joins them before building the index.

## Testing

See [unit-tests](unit-tests.md) for the SearchIndex, App model, and
subprocess-boundary test catalogs. Subprocess tests use a fake `rg`
shell script and a PTY (via `github.com/creack/pty`) to drive the Bubble
Tea program, because Bubble Tea's cancelable reader uses epoll, which
requires a terminal file descriptor. Issue #4 extended the harness with
a controllable blocked fake rg (readiness handshake + indefinite block),
reap-evidence side channel (`VRG_TEST_REAP`), termios snapshot/restore
assertions, display-restoration sequence checks, gate injection
(`VRG_TEST_GATE`), and controlled-failure injection
(`VRG_TEST_FAIL_TRIGGER` / `VRG_TEST_FAIL_DIAGNOSTIC`). Issue #11
extended the harness with the application-side collection
acknowledgement side channel (`VRG_TEST_COLLECT_ACK`), the diagnostic
emission trigger (`VRG_TEST_DIAGNOSTIC_TRIGGER` /
`VRG_TEST_DIAGNOSTIC_TEXT`), and replay-ordering assertions.
