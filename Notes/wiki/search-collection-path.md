# Search collection path (Issue #3)

The ripgrep execution and result-collection pipeline delivered by
[Issue #3](../issues/003-spawn-rg-collect-results-searching-screen.md),
replacing the Issue #2 stub with real subprocess execution, JSON
stream collection, and an interim searching/summary TUI. Relevant PRD
sections: *Implementation Decisions → Invocation and child arguments*,
*Module Design → CLI / SearchIndex / App*, and *Testing Decisions →
CLI / SearchIndex / App*.

## Process boundary

`cmd/vrg/main.go` is the sole process boundary. `runSearch`:

1. Resolves the working directory with `os.Getwd()`.
2. Constructs `exec.Command("rg", res.ChildArgs...)` using the protected
   child argv from the [CLI contract](cli-flag-forwarding.md) and sets
   `rgCmd.Dir` to the working directory.
3. Creates stdout and stderr pipes via `StdoutPipe`/`StderrPipe`.
4. Starts ripgrep with `rgCmd.Start()`. A start failure (rg not on PATH,
   exec error) prints a sanitized diagnostic to stderr and returns exit
   2 without entering the TUI.
5. Constructs an `app.Model` with `app.WithProcess` injecting the running
   process and its pipes.
6. Runs the Bubble Tea program with `tea.NewProgram(model,
   tea.WithOutput(stdout))`.
7. After the program exits, inspects the final model state: if
   `StateStartFailed` and a diagnostic is present, prints it to stderr.
8. Returns the model's exit code (0 for normal quit, 130 for Ctrl-C/quit
   during search, 2 for failure).

The library packages never call `os.Exit` or write directly to
stdout/stderr. All exit-status and stream-destination decisions live in
`cmd/vrg`.

## Ripgrep JSON schema

The `internal/searchindex` package parses ripgrep's `--json` stream.
Recognized event types:

- `begin` — opens a file; carries the path.
- `match` — a matched line; carries path, line number, line text/bytes,
  and submatches.
- `end` — closes a file.
- `summary` — final stream record (currently carries no data we index).
- `context` — recognized but ignored (Issue #3 does not display context
  lines).

Both `text` and `bytes` encodings are supported for paths and line
content. Text and bytes representations of the same logical value produce
identical index entries. Raw non-UTF-8 bytes are retained when ripgrep
reports the `bytes` encoding, so path identity and line content are
preserved exactly.

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
- `Index.Files()` — number of distinct files with matches.

## App model

`internal/app/app.go` implements the Bubble Tea model:

- `Model` — the app state, carrying child argv, working directory,
  process handle, gate channel, file/line counts, diagnostic, and exit
  code.
- `New(childArgs, workdir, opts...)` — constructs a model in the
  searching state.
- `WithProcess(p)` — injects a running ripgrep process (stdout/stderr
  pipes and `*exec.Cmd`).
- `WithGate(ch)` — test seam: holds index preparation until the channel
  is closed or receives, so tests can verify the searching state persists
  after rg exits but before the index is ready.
- `State()` / `ExitCode()` / `Diagnostic()` — accessors for the entry
  point.

### States

- `StateSearching` — initial state while rg is running and results are
  being collected.
- `StateSummary` — collection and index preparation are complete; the
  interim summary is shown.
- `StateStartFailed` — rg could not be started; the entry point should
  print the diagnostic and exit 2.

### Lifecycle

`Init()` returns a `collectResults` command if a process was injected.
`collectResults` runs as a Bubble Tea command:

1. Drains stderr concurrently in a goroutine (`io.Copy` into a buffer) so
   a large stderr stream cannot block ripgrep while stdout collection
   waits.
2. Parses stdout line by line with a `bufio.Scanner` (64 KiB initial,
   64 MiB max buffer) into a `searchindex.Builder`.
3. Waits for the stderr drain goroutine to finish.
4. Waits for ripgrep to exit with `p.Cmd.Wait()`.
5. Holds at the gate if set (test seam).
6. Builds the index and returns a `SearchCompleteMsg` with file and
   line counts.

`Update` handles:

- `SearchCompleteMsg` — transitions to `StateSummary`, records file and
  line counts.
- `SearchFailedMsg` — transitions to `StateStartFailed`, records the
  sanitized diagnostic, sets exit code 2, and quits.
- `tea.KeyPressMsg` — `q` quits (exit 0 from summary, exit 130 from
  searching); Ctrl-C quits with exit 130; escape is ignored.
- `tea.WindowSizeMsg` — records width and height.

`View` renders:

- `StateSearching` — `Searching…`
- `StateSummary` — `N files, M matched lines`
- other states — empty.

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
requires a terminal file descriptor.
