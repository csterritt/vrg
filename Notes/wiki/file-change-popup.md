# File-change pop-up — the instance-keyed filename flash

Issue #15 (`Notes/issues/015-file-change-popup.md`, tasks
`Notes/tasks/015-file-change-popup.md`) delivered the brief centred
pop-up shown when `n`/`p` navigation selects a stop in a different
file, implemented in `internal/app` (`popup.go`, with wiring in
`app.go` and `browse.go`).

PRD cross-references: "Colours, overlays, and key precedence" (the
file-change pop-up bullet and the error-over-pop-up precedence), "File
loading, cache, reload, and selection consistency" (pop-up starts at
selection, not load completion), and "Navigation, viewport, and logical
anchors" (the one-stop no-op that issues no pop-up) in
`Notes/PRD-vrg.md`.

## Lifecycle

- **Starts at selection.** `navigate` opens the pop-up inside the
  `step.FileChanged` branch — the moment the cursor lands in another
  file — so the box is up while the destination still shows
  `Loading…`. A `loadDoneMsg` for that file never restarts it; the
  reveal and placeholder transitions proceed beneath it. Startup
  selection and same-file steps open nothing, and the zero/one-stop
  strict no-ops (`Step.Moved` false) return before the pop-up code.
- **Lifetime: one second or any key press.** `openPopup` mints a fresh
  instance ID (`popupSeq`) per pop-up and returns `popupTick(id)` — a
  `tea.Tick(time.Second, …)` producing `popupExpiredMsg{id}`. The
  expiry is *instance-keyed*: `Update` dismisses only when
  `msg.id == m.popupID`, so a stale instance's late expiry cannot kill
  a newer pop-up, and a dismissed pop-up's timer firing later is inert.
- **Any key dismisses and acts in the same update.** The
  `tea.KeyPressMsg` branch clears `popupID` before routing the key
  normally — `down` scrolls, `n` navigates (opening the *next* file's
  fresh pop-up on another crossing), `q` quits, `Esc` performs only the
  dismissal. The pop-up never delays or swallows input.
- **Cancellation without return.** `openOverlay` — the single route
  every error overlay now takes — clears `popupID`; nothing restores
  it, so a dismissed overlay never reveals the pop-up again. Since
  Issue #31 `openHelp` does the same for the help overlay (see
  [help-overlay.md](help-overlay.md)); Issue #32 owns the combined
  precedence matrix.

## Rendering

`renderPopup` recomputes geometry on every frame: the raw destination
path captured at selection (`popupPath`) is escaped through
`present.Path` — the Issue #6 single-line safe form — left-truncated
with a leading `…` to the frame's interior width, framed by
`theme.Overlay`'s plain single-line border, and centred by `composite`,
the cell-exact splice `renderOverlay` was refactored onto (head and
tail of the underlying row preserved, box rows clipped at the frame
edge). Because centring and truncation derive from the current
`width`/`height` at render time, a `WindowSizeMsg` recentres and
re-truncates the same instance — resize neither dismisses the pop-up
nor restarts its timer. The pop-up composites under the diagnostics
overlay in `View`, matching the PRD's error > help > pop-up precedence.

## The error-overlay trigger this issue introduces

A `loadDoneMsg` failure for the **current** path now opens the
diagnostics overlay (through `openOverlay`) with the
`cannot read <safe path>: <err>` line — the injected current-file
diagnostic the pop-up-cancellation contract is tested against, and the
first slice of the Issue #26 read-failure rules. A non-current
failure still only collects for stderr replay. When an overlay is
already open, `openOverlay` appends the new lines rather than
replacing, preserving the reader's scroll position.

## The `popupTimer` seam

`Model.popupTimer` builds an instance's expiry command; nil selects the
real `tea.Tick`. `browseModel` installs a nil-command timer so tests
unconcerned with the pop-up keep synchronous navigation commands — a
crossing's command is then just the destination's load — while
`popup_test.go`'s `instantPopupTimer` substitutes a command that
resolves to `popupExpiredMsg{id}` immediately, letting `cmdMsgs`
flatten a crossing's `tea.Batch` (load + expiry) without sleeping.
Timers are otherwise driven by injected `popupExpiredMsg` with explicit
instance IDs.

## Tests

`internal/app/popup_test.go` drives `Update` with injected messages:
`TestPopupOpensCentredAtSelection` (centred opening at selection over
`Loading…`, the batched load + instance-keyed expiry in the returned
command), `TestLoadCompletionDoesNotRestartPopup`,
`TestPopupInstanceKeyedExpiry` (fresh-instance minting, stale-expiry
rejection, own-expiry dismissal), `TestPopupKeyDismissalStillActs` /
`TestPopupDismissKeyStillNavigates` / `TestPopupDismissalKeys` (the
any-key contract: `down` scrolls, `n` navigates and opens the next
pop-up, `q` quits, `Esc` dismisses only),
`TestPopupRecentresOnResize` (recentring without a timer restart),
`TestPopupLeftTruncatesLongPath` (leading `…` recomputed per render
size), `TestErrorOverlayCancelsPopup` (cancellation with no return and
an inert stale expiry), and `TestNoPopupWithoutCrossing`.
`sinksafety_test.go` gained the `file-change pop-up` sink row —
`renderPopupSink` names the destination file with the hostile fixture
bytes and asserts the `Path`-escaped form inside the box. See
[unit-tests.md](unit-tests.md).

## Files

- `internal/app/popup.go` — `popupExpiredMsg`, `popupTick`,
  `openPopup`, `renderPopup`.
- `internal/app/browse.go` — `navigate` opens the pop-up on
  `FileChanged` and batches its expiry with the load.
- `internal/app/app.go` — the `popupID`/`popupSeq`/`popupPath`/
  `popupTimer` fields, the `popupExpiredMsg` case, key-press
  dismissal, and `View` compositing.
- `internal/app/overlay.go` — `openOverlay` (open-or-append plus
  pop-up cancellation) and `composite` (the shared centred splice).

See also: [match-navigation.md](match-navigation.md) (the crossing that
triggers it), [error-overlay-and-fatal-outcomes.md](error-overlay-and-fatal-outcomes.md)
(the overlay that cancels it), [safe-presentation.md](safe-presentation.md)
(the path escaping and its sink-safety row),
[destination-reveal.md](destination-reveal.md) (the reveal running
beneath it), [browse-tracer.md](browse-tracer.md) (the frame it
composites over).
