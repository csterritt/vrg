package app

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
)

// hostileNames are the filename byte classes Issue #47 pins — an
// embedded newline, which a raw PathError passthrough would turn into a
// forged diagnostic boundary, a tab, an invalid UTF-8 byte, and an ESC.
// Each is a real on-disk name created under the test's temp dir.
var hostileNames = []struct {
	name string
	base string
}{
	{"newline", "bad\nname.txt"},
	{"tab", "bad\tname.txt"},
	{"invalid-utf8", "bad\xffname.txt"},
	{"esc", "bad\x1bname.txt"},
}

// heldCallSet holds each listed load-worker call at the load gate: call
// n closes its entered channel, then blocks on its own release — the
// deterministic hold letting the test remove the fixture between
// indexing and the real os.ReadFile. Unlisted calls run straight
// through.
type heldCallSet struct {
	calls atomic.Int32
	holds map[int32]*callHold
}

type callHold struct {
	entered chan struct{}
	release chan struct{}
}

func newHeldCallSet(ordinals ...int32) *heldCallSet {
	h := &heldCallSet{holds: map[int32]*callHold{}}
	for _, n := range ordinals {
		h.holds[n] = &callHold{entered: make(chan struct{}), release: make(chan struct{})}
	}
	return h
}

func (h *heldCallSet) fn() func() {
	return func() {
		if hold, ok := h.holds[h.calls.Add(1)]; ok {
			close(hold.entered)
			<-hold.release
		}
	}
}

// removeFixture deletes the indexed fixture so the gated load's real
// read fails with fs.ErrNotExist — never a permission denial, which
// root and unusual ACLs can bypass.
func removeFixture(t *testing.T, path []byte) {
	t.Helper()
	if err := os.Remove(string(path)); err != nil {
		t.Fatalf("removing fixture %q: %v", path, err)
	}
}

// settleReadFailure consumes the load worker's completion, asserts the
// error is the real os.ReadFile *fs.PathError for path — the ENOENT the
// fixture's removal produced, never an injected error — delivers it to
// Update, and asserts the collected diagnostic is exactly one line:
// "cannot read <EscapePath(path)>: <escaped unwrapped cause>" with the
// raw path bytes never repeated in the reason. It returns the expected
// diagnostic line.
func settleReadFailure(t *testing.T, m *model, worker <-chan tea.Msg, path []byte) string {
	t.Helper()
	msg := <-worker
	flm, ok := msg.(fileLoadedMsg)
	if !ok {
		t.Fatalf("load worker produced %#v, want fileLoadedMsg", msg)
	}
	if !bytes.Equal(flm.path, path) {
		t.Fatalf("completion path = %q, want the requested %q", flm.path, path)
	}
	var pe *fs.PathError
	if !errors.As(flm.err, &pe) || !errors.Is(flm.err, fs.ErrNotExist) || pe.Path != string(path) {
		t.Fatalf("read failure = %#v, want the real *fs.PathError (fs.ErrNotExist) for %q", flm.err, path)
	}
	m.Update(msg)
	if !m.failed[string(path)] {
		t.Fatal("the failed read left no failure record")
	}
	if len(m.diags) == 0 {
		t.Fatal("the failed read collected no diagnostic")
	}
	want := "cannot read " + safepresentation.EscapePath(path) + ": " +
		safepresentation.EscapePath([]byte(pe.Err.Error()))
	if strings.Contains(want, "\n") {
		t.Fatalf("the expected diagnostic spans lines: %q", want)
	}
	if strings.Contains(want, string(path)) {
		t.Fatalf("the diagnostic repeats the raw path bytes %q: %q", path, want)
	}
	if got := m.diags[len(m.diags)-1]; got != want {
		t.Fatalf("collected diagnostic = %q, want the single line %q", got, want)
	}
	return want
}

// assertOverlayDiag asserts the open error overlay is exactly the given
// diagnostic lines: the stored text joins them with real boundaries —
// a forged boundary would add a line — and the wrapped row set at a
// generous interior holds one row per line.
func assertOverlayDiag(t *testing.T, m *model, want ...string) {
	t.Helper()
	if !m.overlayOpen {
		t.Fatal("the current-file read failure did not open the overlay")
	}
	if got := m.overlay.text; got != strings.Join(want, "\n") {
		t.Fatalf("overlay text = %q, want exactly %q", got, want)
	}
	if got := m.overlay.rows(1000); !slices.Equal(got, want) {
		t.Fatalf("overlay rows = %q, want exactly %q", got, want)
	}
}

// navLoadWorker sends a file-crossing key and launches the returned
// batch's first leaf — the destination's load worker — returning its
// message channel.
func navLoadWorker(t *testing.T, m *model, key tea.KeyPressMsg) <-chan tea.Msg {
	t.Helper()
	_, nav := m.Update(key)
	batch, ok := nav().(tea.BatchMsg)
	if !ok || len(batch) < 1 {
		t.Fatalf("crossing batch = %#v, want the load leaf first", nav())
	}
	return runCmd(batch[0])
}

// Issue #47, initial-load site: a real os.ReadFile failure — the
// indexed fixture is removed while its load is held at the gate —
// produces exactly one diagnostic line whatever bytes the filename
// carries: the EscapePath-escaped path plus the unwrapped PathError
// cause, identical through the overlay row set and the stderr replay.
func TestInitialLoadFailureSingleLineDiag(t *testing.T) {
	for _, tc := range hostileNames {
		t.Run(tc.name, func(t *testing.T) {
			gate := newHeldLoads()
			m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: gate.fn()})
			idx := navIndex(t, []navFile{{
				name:    "zz-" + tc.base,
				content: "hit\n",
				stops:   []navStop{{line: 1, start: 0, end: 3}},
			}})
			path := idx.Files[0].Path
			worker := runCmd(startBrowse(t, m, idx))
			<-gate.entered // the load holds ahead of the real read
			removeFixture(t, path)
			close(gate.release)

			want := settleReadFailure(t, m, worker, path)
			assertOverlayDiag(t, m, want)
			if got := replayOutput(t, m); got != want+"\n" {
				t.Fatalf("replay = %q, want the one line %q", got, want)
			}
		})
	}
}

// Issue #47, r reload site: the explicit reread hits the same
// construction — the fixture is removed under the gate so the real
// reread fails, and the failure is again exactly one escaped-path line
// in the collection, the overlay, and the replay.
func TestReloadFailureSingleLineDiag(t *testing.T) {
	for _, tc := range hostileNames {
		t.Run(tc.name, func(t *testing.T) {
			gate := newHeldNthLoad(2) // hold only the reload's read
			m := newTestModel(fakeChild{res: Result{Code: 0}}, options{loadGate: gate.fn()})
			idx := navIndex(t, []navFile{{
				name:    "zz-" + tc.base,
				content: "hit\n",
				stops:   []navStop{{line: 1, start: 0, end: 3}},
			}})
			path := idx.Files[0].Path
			finishLoad(t, m, startBrowse(t, m, idx)) // call 1 loads fine

			_, cmd := m.Update(keyR)
			if cmd == nil {
				t.Fatal("r minted no reload command")
			}
			worker := runCmd(cmd)
			<-gate.entered // call 2 holds ahead of the real reread
			removeFixture(t, path)
			close(gate.release)

			want := settleReadFailure(t, m, worker, path)
			assertOverlayDiag(t, m, want)
			if got := replayOutput(t, m); got != want+"\n" {
				t.Fatalf("replay = %q, want the one line %q", got, want)
			}
			if v := viewText(m); !strings.Contains(v, "(unreadable)") {
				t.Fatalf("view = %q, want (unreadable) after the failed reload", v)
			}
		})
	}
}

// Issue #47, re-entry retry site: crossing back into a failed file
// shows the prior single-line failure while exactly one retry is
// minted; the retry's real read failure — the fixture stays removed —
// lands as one more occurrence of the identical single line, appended
// to the open overlay and the collection, with the replay carrying both
// verbatim.
func TestReentryRetryFailureSingleLineDiag(t *testing.T) {
	gate := newHeldCallSet(2, 3) // b's first load and its re-entry retry
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{
		loadGate:   gate.fn(),
		popupTimer: popupStubTicks.popupTimer,
	})
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
		{name: "zz-bad\nname.txt", content: "hit\n", stops: []navStop{{line: 1, start: 0, end: 3}}},
	})
	pathB := idx.Files[1].Path
	finishLoad(t, m, startBrowse(t, m, idx)) // call 1: a loads

	// Cross into the hostile-named file: its load holds at the gate and
	// the fixture's removal makes the real read fail.
	worker := navLoadWorker(t, m, keyN)
	<-gate.holds[2].entered
	removeFixture(t, pathB)
	close(gate.holds[2].release)
	want := settleReadFailure(t, m, worker, pathB)
	assertOverlayDiag(t, m, want)

	m.Update(keyEsc)
	m.Update(keyP) // back to a — cached, no load minted

	// Re-entry shows the prior failure while the retry is in flight.
	worker = navLoadWorker(t, m, keyN)
	if _, ok := m.loading[string(pathB)]; !ok {
		t.Fatal("re-entering the failed file minted no retry")
	}
	if !m.overlayOpen || m.overlay.text != want {
		t.Fatalf("re-entry overlay = %q, want the prior failure %q", m.overlay.text, want)
	}

	<-gate.holds[3].entered // the retry holds ahead of its real read
	close(gate.holds[3].release)
	if got := settleReadFailure(t, m, worker, pathB); got != want {
		t.Fatalf("retry diagnostic = %q, want the identical %q", got, want)
	}
	assertOverlayDiag(t, m, want, want)
	assertReplayLines(t, m.diags, []string{want, want})
	if got := replayOutput(t, m); got != want+"\n"+want+"\n" {
		t.Fatalf("replay = %q, want the two identical lines", got)
	}
}
