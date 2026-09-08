## Issue 9: Error overlay, fatal search outcomes (exit 2), and the outcome-transition matrix

**Type**: AFK
**Blocked by**: Issue 8

### Parent PRD

`Notes/PRD-vrg.md`

### What to build

The modal error overlay, the fatal rows of the outcome table, and the **single outcome/transition test matrix** that later issues extend.

- Stderr captured by Issue 3's concurrent dual-pipe drainage is included in diagnostics regardless of exit code. If a failed process supplies no stderr, generate a diagnostic naming the exit code or signal.
- Fatal conditions: rg exits other than 0/1, dies by signal, or **stream integrity fails** (missing summary, missing/orphaned/inconsistent `begin`/`end` pairs, lost completion metadata). Integrity and process success are assessed separately; a summary alone is a complete zero-result stream.
- **Lifecycle validation** follows the transition matrix below. Path identity is compared on decoded raw path bytes, so `text` and `bytes` forms of the same path are the same file; a `begin`/`match`/`end` path disagreement manifests as whichever event is orphaned. Multiple files may be open simultaneously (rg may interleave events across files during parallel search); per-path state is tracked independently. `context` records participate in no lifecycle validation.

| Transition | Disposition |
|---|---|
| `begin(P)` while P is not open | Valid — P opens |
| `begin(P)` while P is already open | Integrity failure (duplicate `begin`) |
| `match(P)` while P is open | Valid — indexes under P |
| `match(P)` while P is not open (never opened, or after its `end`) | Integrity failure (orphaned/inconsistent match); P's matches are retained with incomplete metadata |
| `end(P)` while P is open | Valid — P closes; non-null `binary_offset` → binary exclusion of P |
| `end(P)` while P is not open | Integrity failure (orphaned/duplicate `end`) |
| `context(P)` in any position | Ignored — no lifecycle effect |
| P still open when the stream ends | Integrity failure (missing `end`); P's matches are retained with incomplete metadata |
| Exactly one `summary`, as the final record | Valid — a summary alone is a complete zero-result stream |
| `summary` missing | Integrity failure |
| A second `summary` | Integrity failure |
| Any record after `summary` | Integrity failure (also skipped/counted if the record is itself malformed) |
| Trailing unterminated record | Both — counted malformed and the stream is incomplete |

  Violations of Issue 3's per-record schema matrix remain skipped-and-counted malformed records; a safely skipped match record does not by itself make otherwise intact lifecycle metadata incomplete.
- **Binary exclusion precedence**: a `match(P)` arriving after a binary-excluding `end(P)` (non-null `binary_offset`) is an integrity failure (orphaned match after `end`), and the late match is **not retained** — binary exclusion takes precedence over the general retention rule for orphaned matches. The file remains binary-excluded from the file list, so no retained match from it is browsable.
- With usable results: browse with the error overlay open; `q`/`Esc` dismiss to browsing; eventual `q` exits 2.
- Without usable results: error overlay; dismissal (`q` **or** `Esc`) exits 2. `Esc` never exits from a base state; this is the one case where `Esc` terminates — dismissing a fatal no-results overlay with `Esc` terminates with status 2 because there is no underlying state.
- Non-fatal stderr on rg 0/1 with usable results: browse with a warning overlay, eventual exit 0.
- Non-fatal stderr on rg 0/1, complete stream, **zero** usable results: warning overlay first; dismissal goes to the no-results screen; `q` there exits 1.
- Overlay: base colours, single-line border, `up`/`down` scroll, `q`/`Esc` dismiss, `ctrl+c` exits 130, other keys ignored. Text wraps to interior width, including unbroken strings. Overlay text is routed through the Issue 6 utility (add the error overlay to the sink-safety table).
- The fixed exit status is decided once searching completes and never changes afterwards, **except** that `ctrl+c` still overrides it with 130.
- Frame the outcome decision as a pure, testable function of (process result, integrity, usable-result count, record-loss counts, warning diagnostics) returning (initial presentation, post-dismissal state, exit status). Issue 10 adds the record-loss inputs; Issues 26/29/30 assert their conditions do not alter it.

See PRD *Outcome and exit-status contract* (table rows 4–5, 7–8 and bullets), *Result index…* (integrity bullets), *Colours, overlays, and key precedence* (error overlay bullet).

### How to verify

- **Manual**:
  1. Fake rg that emits two valid matches then exits 3 with stderr "boom" → browse view with overlay containing "boom"; `Esc` dismisses; `q` exits 2.
  2. Fake rg that exits 2 with no output → overlay naming exit code 2; `q` exits 2. Repeat with `Esc` → also exits 2.
  3. Fake rg killed by SIGKILL mid-stream → overlay names the signal.
  4. Fake rg that writes "warn" to stderr and a summary-only stream, exit 1 → warning overlay; `Esc` → "No results found"; `q` → exit 1.
- **Automated**:
  - **Outcome matrix** (table-driven, App model): one row per outcome-table row, each with and without usable results where meaningful, for both rg exit 0 and 1 where the row allows it. Must include at minimum: rg 0 clean browse → 0; rg 1 with retained results and complete stream → browse, 0 (anomalous-exit-1 case); rg 1 empty → no-results, 1; fatal code with results → browse+overlay, dismiss → browse, `q` → 2; fatal code without results → overlay, `q` → 2 and separately `Esc` → 2; signal death with/without results; missing summary with valid matches → 2; orphaned `end` → 2; stderr warning + results → warning overlay, 0; stderr warning + zero results → warning overlay → no-results → 1; all-binary after warning → warning overlay → no-results with count → 1; `ctrl+c` after search completion in browse, no-results, and open-overlay states → 130. Each row asserts the initial presentation, the post-dismissal presentation (and which of `q`/`Esc` was used), and the final exit status. The matrix is a **single table-driven test**; later issues extend it with new rows rather than duplicating the decision.
  - Lifecycle fixtures covering every row of the transition matrix above (duplicate `begin`, orphaned `match`/`end`, `match` after `end`, records after `summary`, second `summary`, interleaved open files with valid pairing, `text`/`bytes` path-identity agreement).
  - Overlay key routing (scroll, dismiss, ignored keys, `ctrl+c`); long unbroken diagnostic wraps within the border; hostile fixture text in the overlay passes the Issue 6 sink-safety check.
  - **Subprocess/PTY** (Issue 4 harness): fake rg exiting non-zero after a handshake; and a **stderr-content fixture**: fake rg writes ≥ 1 MiB to stderr (well over pipe capacity) interleaved with a valid stdout stream, with a handshake confirming it finished writing both before exit. The dual-pipe drainage and deadlock-freedom assertions are owned by Issue 3; here assert that the stdout stream is complete, the captured stderr is included in diagnostics, and the overlay contains the head and tail of the stderr text.

### Acceptance criteria

- [ ] Given rg exits with a code other than 0/1 or by signal and usable results exist, then browsing starts with an error overlay and the eventual `q` exits 2.
- [ ] Given the same failure with no usable results, then dismissal with either `q` or `Esc` exits 2.
- [ ] Given a stream lacking a valid summary or with unpaired lifecycle events, then integrity fails and the fatal rules apply.
- [ ] Given rg exits 0/1 with stderr output and usable results, then a warning overlay is shown and the eventual exit is 0.
- [ ] Given rg exits 0/1, stderr output, complete stream and zero usable results, then a warning overlay is shown, dismissal shows the no-results screen, and `q` exits 1.
- [ ] Given rg exits 1 with a complete stream and retained results, then browsing proceeds and the eventual exit is 0.
- [ ] Given a failed process with empty stderr, then the overlay names the exit code or signal rather than being empty.
- [ ] Given an error overlay, when any key other than `up`/`down`/`q`/`Esc`/`ctrl+c` is pressed, then it is ignored.
- [ ] Given a fixed search-derived status, when `ctrl+c` is pressed later in any state, then the exit status is 130.
- [ ] Given a fake rg that writes more than pipe capacity to stderr while streaming stdout, then the captured stderr is included in diagnostics and the overlay contains its head and tail (deadlock-freedom itself is asserted by Issue 3).

### User stories addressed

- User story 15: partial results browsable alongside an error overlay; exit 2
- User story 16: fatal search with no results shows overlay and exits 2
- User story 19: missing completion metadata reported
- User story 21 (part): nonfatal stderr diagnostics with no results shown before the no-results screen

---
