# Search collection: spawn, drainage, searching screen (Issue #3)

The first real Bubble Tea application path, delivered by
[Issue #3](../issues/003-spawn-rg-collect-results-searching-screen.md):
spawn `rg`, collect its JSON event stream into the SearchIndex, hold a
"Searching…" screen for the whole of collection **and** post-exit index
preparation, then move on to browsing (the Issue #3 interim summary was
replaced by the [Issue #5 browse view](browse-tracer.md)). Relevant PRD sections:
*Implementation Decisions → Invocation and child arguments* (bullets
7–9), *Result index, records, and stream integrity* (bullets 1–3), and
*Module Design → SearchIndex / App* in [`Notes/PRD-vrg.md`](../PRD-vrg.md).

## Child argv and working directory

`cmd/vrg` hands `cli.Result.ChildArgs` — the protected vector
`--json --no-config <ordered user flags> -- <pattern> <root>` built by the
[Issue #2 CLI](cli-flags-child-argv.md) — to `app.Run`, which spawns `rg`
from `PATH` with that argv. The child runs with `cmd.Dir` set to the
invocation working directory (`os.Getwd()`), and relative result paths
resolve against that same directory — never by prepending the supplied
root again, and never canonicalized.

## Dual-pipe drainage

`internal/app/rg.go`'s `spawn` wires both `StdoutPipe` and `StderrPipe`,
starts the child, then a collector goroutine drains **both pipes
concurrently for the whole child lifetime**: two `io.Copy` goroutines
read to EOF before `cmd.Wait()` reaps the child (the correct ordering —
`Wait` must not run before the reads complete). Neither pipe can fill and
block rg; stderr bytes are buffered without backpressure. Terminating the
child closes the pipes, so the copies end promptly rather than waiting
for further output — the structure Issue #4's cancellation rides on.
Classification and presentation of the captured stderr remain Issue #9's.

## Searching state and the test gate

The model (`internal/app/app.go`) opens in `stateSearching`, rendering
"Searching…". `Init` returns one command that runs entirely off the UI
update path: `child.Wait()` (blocks until rg exits **and** both pipes are
drained), then the optional collect-acknowledgement and preparation gate,
then `searchindex.Build`. Because the whole sequence is a single command,
the model provably stays searching until the index is ready — even after
rg itself has exited. Resize messages are ordinary updates and never
block on collection. `q` while searching — including that gate-held
post-exit window — is Issue #4's cancellation, exiting 130; see
[cancellation-cleanup.md](cancellation-cleanup.md).

Two seams, delivered as `app.Option`s, support tests:

- `WithGate(fn)` — runs after collection completes and before index
  preparation; a test can hold preparation independently of rg exit.
- `WithCollectAck(fn)` — runs once collection completes, before the gate.

`cmd/vrg` wires them from environment variables: `VRG_TEST_GATE=<file>`
holds preparation while the file exists (a polled watcher), and
`VRG_TEST_COLLECT_ACK=<file>` appends a line when collection completes,
so a test can observe "rg exited, stream collected, preparation still
held". Issue #4 added `VRG_TEST_REAP` and `VRG_TEST_FAIL` (see
[cancellation-cleanup.md](cancellation-cleanup.md)). Issue #45 will move
these env-var seams behind the `vrg_testhooks` build tag.

## Search completion and start failure

Completion delivers `searchDoneMsg` carrying the collected result and the
prepared index; Issue #3 transitioned to an interim `stateSummary`
rendering `N files, M matched lines`. Issue #5 replaced that screen:
when the prepared index's usable results are nonzero the model
transitions to `stateBrowse` and starts loading the first file — see
[browse-tracer.md](browse-tracer.md). `q` in browse returns `tea.Quit`
with the fixed exit status 0. Issue #8 added the empty branch: zero
usable results enters `stateNoResults` — the centred "No results found"
screen, `q` → exit 1 — see
[no-results-screen.md](no-results-screen.md).

A start failure (rg absent, exec error) is detected in `app.Run` **before**
`tea.NewProgram` runs: a sanitized single-line diagnostic
(`vrg: cannot start ripgrep: …`, the error text single-line-escaped via
`safepresentation.EscapePath`) goes to stderr and the
process exits 2 — no TUI is entered.

## SearchIndex record rules

`internal/searchindex` consumes the newline-delimited JSON stream.

- **Records**: `ParseRecord` decodes one record and classifies it —
  `begin`, `match`, `end`, `summary`, `context` (known, data ignored), a
  string `type` outside the five (`KindUnknown`), or `KindMalformed`
  (invalid JSON, invalid base64, missing/non-string `type`, or a known
  event failing the required-field/range matrix). Validation reads
  exactly the required fields; matrix-ignored fields such as
  `absolute_offset` and `stats` are never consulted.
- **Encodings**: `{"text": …}` and `{"bytes": base64}` both decode to raw
  bytes for paths, lines, and submatch text; identical bytes are the same
  value, so a `text` record and a `bytes` record merge by content.
- **Merging**: match records sharing a raw path and `line_number` merge
  into one `Stop`; submatches sort by `(start, end)`; overlapping
  submatches are all retained and `Highlights` holds their union
  coverage (touching ranges merge).
- **Retention**: raw path bytes, line number, decoded line bytes, submatch
  byte ranges, and recorded submatch bytes are all kept.
- **Ordering**: files sort by unsigned raw path bytes (`bytes.Compare`),
  so high/invalid-UTF-8 bytes order deterministically after ASCII;
  stops sort by ascending line number.
- **Resolution**: relative result paths are byte-joined onto the
  invocation working directory (`wd + "/" + path`) without
  canonicalization — interior `.`/`..` elements stay literal.

Binary exclusion landed with Issue #8: an `end` record carrying a
non-null `binary_offset` drops that file's collected stops and counts it
once in `Index.BinaryExcluded`, and `Index.UsableResults()` reports
retained stops after filtering — see
[no-results-screen.md](no-results-screen.md). Issue #9 landed the
lifecycle/integrity accounting — begin/end pairing, summary
completeness, and `Index.Integrity()` — see
[error-overlay-and-outcomes.md](error-overlay-and-outcomes.md); the
malformed/oversized/unknown skip counts and the 64 MiB record limit
landed with Issue #10 — see
[record-robustness.md](record-robustness.md). Issue #13 landed the
circular matched-line cursor (`cursor.go`): `Index.Cursor()`/
`Next()`/`Prev()` walking the prepared stops in path-then-line order
with a `Move{Wrapped, FileChanged}` report — see
[match-navigation.md](match-navigation.md).

## Tests

See [unit-tests.md](unit-tests.md): `internal/searchindex/index_test.go`
covers the record/index contracts including binary exclusion;
`internal/app/app_test.go` covers the model lifecycle,
`noresults_test.go` the no-results outcome (Issue #8), `browse_test.go`
the browse view, and `rg_test.go` the spawn seam; `cmd/vrg/search_test.go` drives the real binary on a pty
(`runVrgWithQuit`) for the exact child argv/workdir, the ≥1 MiB
dual-pipe backpressure fixture, stderr capture, the gate-held searching
state, and `TestStartFailureExit2`.
