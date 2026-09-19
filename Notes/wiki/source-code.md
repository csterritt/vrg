# Source code

Catalog of Go source under `cmd/` and `internal/`. Module path: `vrg`.

## cmd/

- `cmd/vrg/main.go` — thin process boundary. `run` calls `cli.Parse` with
  `os.Stat` injected and maps the explicit result kind to stream/status:
  help → exit 0 (help already on stdout), usage error → sanitized
  diagnostic + usage on stderr, exit 2, search → `app.Run` with the
  protected child argv and the invocation working directory. It also
  wires the `VRG_TEST_*` seam env vars (`VRG_TEST_GATE`,
  `VRG_TEST_COLLECT_ACK`, `VRG_TEST_REAP`, `VRG_TEST_FAIL`,
  `VRG_TEST_LOAD_GATE`, `VRG_TEST_DIAG_ACK`) into `app.Option`s — see
  [search-collection.md](search-collection.md),
  [cancellation-cleanup.md](cancellation-cleanup.md),
  [browse-tracer.md](browse-tracer.md), and
  [stderr-replay.md](stderr-replay.md).

## internal/cli

- `internal/cli/cli.go` — the CLI module and sole `mow.cli` v1.2.0
  adapter. Contains the shared `optionDecls`/`argDecls` table (parser
  config + raw-token recognition + ordered flag records + generated
  help), the ordered `scanArgs` preflight with combined-short expansion
  and cumulative unrestricted counting, `renderHelp`, `checkRoot`, and
  the `Result`/`Kind`/`ErrorKind`/`Env` contract including
  `Result.ChildArgs` (the protected `rg` argv). Sanitization routes
  through `internal/safepresentation` (the Issue #1 `Escape` is
  replaced): usage diagnostics embed `EscapePath`-escaped operands and
  pass through `EscapeDiagnostic`, as does generated help. See
  [cli-foundation.md](cli-foundation.md),
  [cli-flags-child-argv.md](cli-flags-child-argv.md), and
  [safe-presentation.md](safe-presentation.md).

## internal/searchindex

- `internal/searchindex/record.go` — the per-record JSON parser and the
  Issue #3 schema matrix: `Kind` classification (`begin`/`match`/`end`/
  `summary`/`context`/`KindUnknown`/`KindMalformed`), `text`/base64
  `bytes` decoding, required-field and range validation.
- `internal/searchindex/index.go` — the navigation index: `Index.Feed` /
  `FeedTail` / `Prepare` / `Build`, same-path/same-line `Stop` merging,
  submatch `(start, end)` ordering, union `Highlights`, unsigned
  raw-path ordering, and working-directory path resolution without
  canonicalization. Issue #8 added binary exclusion — an `end` with
  non-null `binary_offset` drops the file's stops and counts it once in
  `Index.BinaryExcluded` — and `UsableResults()`, the retained-stop
  count the outcome logic consumes. Issue #9 added lifecycle
  validation: per-path open tracking on decoded raw bytes, the
  transition matrix (duplicate/orphaned begin/end, retained incomplete
  matches, post-summary records, the unterminated tail), binary-
  exclusion precedence over orphan retention, `File.Incomplete` for
  partially-lifecycled files, and `Integrity().Complete` — stream
  integrity assessed separately from process success. Issue #10 added
  the skip counters `Malformed`/`Unknown` — classified per record,
  unqualified by position — and `Build`'s bounded scan that discards
  records over `MaxRecordBytes` through their next newline. See
  [no-results-screen.md](no-results-screen.md),
  [error-overlay-and-outcomes.md](error-overlay-and-outcomes.md), and
  [record-robustness.md](record-robustness.md).
- `internal/searchindex/cursor.go` — Issue #13's circular matched-line
  cursor: `Cursor{File, Stop}` positions, `Index.Cursor()` reporting
  the position (absent on an empty index), `Next`/`Prev` stepping one
  stop in path-then-line order with wrap at both ends, and the
  `Move{Wrapped, FileChanged}` report — file change detected by
  comparing the destination to the departing file so a one-file wrap
  reports `Wrapped` alone. Zero or one total stops makes both keys
  strict no-ops. See [match-navigation.md](match-navigation.md).
- `internal/searchindex/oversized.go` — Issue #10's `MaxRecordBytes`
  (64 MiB payload limit) and the oversized-record accounting:
  `feedOversized` counts the record, recovers its path best-effort for
  diagnostics via `recoverOversizedPath` — a bounded `json.Decoder`
  token pass over the first `MaxRecordBytes` for `type` and
  `data.path` — and flags an after-`summary` arrival as an integrity
  failure. See [record-robustness.md](record-robustness.md).
- `internal/searchindex/doc.go` — package comment.

## internal/app

- `internal/app/rg.go` — the child-process seam: `spawn` runs `rg` from
  `PATH` with `cmd.Dir` set to the invocation working directory and
  drains both stdout and stderr concurrently for the child's whole
  lifetime; `Child`/`Result`/`StartFunc` are the boundary types, with
  `Child.Terminate` and an idempotent `Wait` backing the cleanup path.
  Issue #11 added `Child.Diags`: `drainStderr` line-splits stderr into
  a mutex-guarded pending queue (drainage never blocks on a consumer)
  and `feedDiags` forwards it onto the channel in order, closed at EOF.
- `internal/app/app.go` — the Bubble Tea model and `app.Run`: the
  "Searching…" state covering collection and post-exit index
  preparation, the `WithGate`/`WithCollectAck`/`WithDiagAck` test
  seams, the `searchDoneMsg` outcome decision — `stateBrowse`, the
  Issue #8 `stateNoResults` alternative with its "(N binary files
  skipped)" suffix, or Issue #9's `stateOverlayOnly` fatal
  presentation — the modal-overlay key routing, the sanitized
  start-failure diagnostic with exit 2 before the TUI, and the Issue #4
  surface: `ctrl+c`/`q` cancellation to 130, the `quitCmd`/`reapChild`
  cleanup boundary, `WithFailFunc`/`WithReapReport`, the `quitting`
  discard of late completions, alt-screen views, `ErrInterrupted` →
  130, and the Issue #7 `c` key toggling `m.theme` between the dark
  and light schemes. The search-derived `status` is fixed at
  completion; only `ctrl+c` overrides it to 130. Issue #10's
  `recordWarnings` composes the "N unrecognised record types skipped"
  warning from the index's `Unknown` count. Issue #12 adds the browse
  scroll-key route (`stateBrowse` + `isScrollKey` → `scrollBy`), the
  per-file `vps`/`rows` maps, saved-viewport re-clamping on resize and
  on load completion. Issue #14's `fileLoadedMsg` path also runs
  `model.reveal` when the completed path is the current file — the
  startup-after-load reveal trigger. Issue #13 adds the `n`/`p` browse
  route to
  `navigate` and drops `model.cur`: the current file and current
  matched line derive from the index's matched-line cursor. Issue #15
  adds the `popupID`/`popupSeq` pop-up state, the `popupExpireMsg`
  stale-instance-rejecting expiry case, any-key dismissal before the
  key switch, `View` compositing the pop-up over the base frame and the
  overlay over both, and the `options.popupTimer` test seam. Issue #11 adds the session
  diagnostic collection `model.diags`: `collectDiags` appends sanitized
  lines as `Update` processes `stderrLineMsg` (forwarded from
  `Child.Diags()` by a `prog.Send` goroutine in `Run`),
  `searchDoneMsg`, `fileLoadedMsg` failures, and `failMsg`; after the
  program returns and the child is reaped, `replayDiags` writes the
  collection to stderr exactly once, in order — the common
  post-restoration writer that also replaced the controlled-failure
  direct write. See
  [cancellation-cleanup.md](cancellation-cleanup.md),
  [theme.md](theme.md),
  [no-results-screen.md](no-results-screen.md),
  [error-overlay-and-outcomes.md](error-overlay-and-outcomes.md),
  [record-robustness.md](record-robustness.md), and
  [stderr-replay.md](stderr-replay.md).
- `internal/app/outcome.go` — Issue #9's pure outcome decision:
  `DecideOutcome` maps `OutcomeInput` (process `Result`, stream
  `Integrity`, usable-results count, `RecordLoss`, caller `Warnings`)
  to the presentation, the escaped overlay lines, the dismiss-exits
  flag, and the fixed status — the whole PRD outcome table. Issue #10
  filled `RecordLoss` (`Malformed`, `Oversized`, the recoverable
  `Paths`) and added its row: a complete stream with skipped records
  and zero usable results — assessed after all filtering — is the
  fatal overlay-only outcome. `outcomeDiagnostics` composes the overlay
  lines in the universal order — captured stderr or a generated
  code-or-signal line for a silent failed process, then the integrity
  note, then `recordLossLines` (the malformed/oversized counts plus
  "oversized record skipped for \<path\>" lines), then warnings — all
  through `safepresentation.EscapeDiagnostic`/`EscapePath`. Issue #11
  split the composition into `processDiags` (the process component) and
  `tailDiags` (integrity + record loss + warnings) so the session
  collection shares it without re-collecting incrementally delivered
  stderr.
- `internal/app/overlay.go` — Issue #9's modal error overlay:
  grapheme-boundary `wrapCells` to the interior width (unbroken strings
  split mid-run), the complete wrapped row set scrolled by
  `up`/`down` clamped to `[0, rows − visible]`, and `compositeOverlay`
  centering the single-line bordered box over the base frame. Issue #15
  adds `openOverlay` — the shared open-or-append entry point that also
  clears any live file-change pop-up, which never returns after the
  overlay closes.
- `internal/app/popup.go` — Issue #15's file-change pop-up:
  `popupExpireMsg{id}` and `startPopup` minting a fresh instance ID per
  file crossing with a one-second `tea.Tick` expiry (or the
  `options.popupTimer` test seam), and `compositePopup` — the
  `theme.Overlay`-bordered box centred over the base frame carrying the
  current file's `EscapePath`-sanitized path, left-truncated with a
  leading `…` by `leftTruncate`/`tailCells` on whole grapheme clusters,
  all recomputed from the live terminal size at every render. See
  [file-change-popup.md](file-change-popup.md).
- `internal/app/browse.go` — the Issue #5 browse view: async
  `filebuffer.Load` commands gated by `WithLoadGate`, the
  `loading`/`bufs`/`failed` caches keyed by raw path, `fileLoadedMsg`,
  the full-width filename rule, the fixed-width file list with the
  current entry underlined, the right-justified gutter, inverse-video
  match runs, and the "Loading…"/"(unreadable)" placeholders — all
  styled through `internal/theme` since Issue #7 (base-wrapped frame,
  true-inverse matches, underlined current match and current file).
  Issue #11's `loadDiag` composes the single-line `cannot read
  \<EscapePath(path)\>: \<reason\>` diagnostic a failed load collects
  (`*fs.PathError` contributes only its cause). Issue #12 adds the
  `rowSource` seam the frame render queries per visible row,
  `curKey`/`contentRows`, and `isScrollKey`/`scrollBy` — the
  `up`/`down`/`u`/`d`/`pgup`/`pgdown` handling that scrolls the
  current file's saved per-file viewport clamped to the prepared row
  count and content height, a no-op on placeholders. Issue #13 adds
  `curFile` (the cursor-derived current file read by `curKey`,
  `startLoad`, the list underline, and the filename rule), the
  cursor-derived `curLine` driving `CurrentMatch`, and `navigate` —
  `n`/`p` step the index cursor and issue `startLoad` for the
  destination only on `Move.FileChanged`, with the departing file's
  viewport already saved by scroll write-through. Issue #14 extends
  `rowSource` with `TargetRow`, adds `model.reveal` — the
  starting-viewport sequence (saved `vps` top or first-visit top of
  file, then `Viewport.Reveal`, written back only on a move) — and wires
  it into `navigate`'s actual transitions and the
  current-file `fileLoadedMsg` path in app.go. Issue #15 makes
  `navigate` return `tea.Batch(m.startLoad(), m.startPopup())` on a
  file change — the pop-up starts at selection while the destination
  may still be loading, and load completion never restarts it. See
  [browse-tracer.md](browse-tracer.md), [theme.md](theme.md),
  [stderr-replay.md](stderr-replay.md),
  [file-change-popup.md](file-change-popup.md),
  [viewport-scrolling.md](viewport-scrolling.md),
  [match-navigation.md](match-navigation.md), and
  [destination-reveal.md](destination-reveal.md).
- `internal/app/doc.go` — package comment.

## internal/filebuffer, internal/viewport, internal/theme, internal/safepresentation

- `internal/filebuffer/filebuffer.go` — `Load` reads the file by raw
  path bytes, splits LF/CRLF, maps each line through
  `safepresentation.MapContent`, and produces `Buffer`/`Line` records
  with `GutterWidth`, source `Raw` bytes, and `Highlights` mapped to
  display cells. See [browse-tracer.md](browse-tracer.md).
- `internal/viewport/viewport.go` — `Viewport`, the file panel's
  vertical window (`Top`, `Scroll`, `Clamp`), the scroll-unit helpers
  `HalfPage` and `MaxTop` (the BOF/EOF clamp bound), and `Rows`, the
  prepared rendered-row model built by `Prepare` that the frame render
  slices. Wrap, logical anchors, and horizontal state are later issues.
  See [viewport-scrolling.md](viewport-scrolling.md).
- `internal/viewport/reveal.go` — Issue #14's vertical destination
  reveal: `Target{Line, Cell}` (the display target — the first
  submatch's start cell), `Rows.StopTarget` (stop → target through the
  byte→cell map, with the zero-width marker-cell fallback),
  `Rows.TargetRow` (target → rendered row), and `Viewport.Reveal`
  (visible-target no-scroll, else top = `row − floor(h/3)` clamped to
  `[0, MaxTop]`, reporting whether the viewport moved). See
  [destination-reveal.md](destination-reveal.md).
- `internal/theme/theme.go` — the active scheme's style set: `Dark()`
  (white on black, initially active), `Light()` (black on white), the
  pure `Toggled` flip behind the `c` key, `Plain()` (the no-style
  composition path for sink-safety tests), and the decorators `Base`,
  `Gutter`, `Match` (true-inverse colours), `CurrentMatch` (inverse +
  underline), `Indicator`, `FilenameRule`, `FileList`, `CurrentFile`
  (base + underline), and `Overlay` (base colours inside a plain
  single-line border). See [theme.md](theme.md).
- `internal/safepresentation/safepresentation.go` — `EscapePath`,
  `MapContent`, `Mapped`/`Cell`/`CellsCovering` byte→cell maps.
- `internal/safepresentation/diagnostic.go` — `EscapeDiagnostic`:
  real line boundaries preserved (CRLF normalizes to LF), tabs expand
  to eight-column stops, other controls escaped; embedded external
  strings are `EscapePath`-escaped first.
- `internal/safepresentation/cellwidth.go` — `CellWidth` and the
  package-internal rune decoders (Issue #39 boundary).
- `internal/safepresentation/sinktest/sinktest.go` — test-support
  package (imported only by `_test.go` files): the shared hostile
  `Fixtures`, the `Sink` row type, the `AssertRawOutput`/
  `AssertPayloadNotEscaped` raw-output assertions, and the `Run`
  table driver. See [safe-presentation.md](safe-presentation.md).
