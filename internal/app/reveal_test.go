package app

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// longFile is a 250-line fixture with matched lines at the given
// 1-based numbers; the match covers each line's trailing digits so
// every stop's match is a distinct styled run.
func longFile(lines ...int64) navFile {
	stops := make([]navStop, len(lines))
	for i, n := range lines {
		stops[i] = navStop{line: n, start: 10, end: 10 + len(strconv.FormatInt(n, 10))}
	}
	return navFile{name: "long.txt", content: numberedContent("long", 250), stops: stops}
}

// wide loads the browse view at 160x24 — wide enough that the fixture
// paths' file list leaves a full-width panel — on the plain theme so
// content text stays contiguous for row assertions. The content height
// is 23 rows, so one-third placement is content row 7.
func wide(t *testing.T, m *model, idx *searchindex.Index) {
	t.Helper()
	m.theme = theme.Plain()
	cmd := startBrowse(t, m, idx)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 24})
	finishLoad(t, m, cmd)
}

// Once the startup file's content loads, the first stop's target row
// is revealed: a hidden target lands at zero-based row
// floor(content height / 3).
func TestStartupRevealPlacesHiddenTarget(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{longFile(200)})
	wide(t, m, idx)

	key := string(idx.Files[0].Path)
	if got := m.vps[key].Top(); got != 192 {
		t.Fatalf("startup reveal top = %d, want 192 (row 199 at content row 7)", got)
	}
	if row := frameRow(t, m, 8); !strings.Contains(row, "long line 200") {
		t.Fatalf("frame row 8 = %q, want the target line at content row 7", row)
	}
}

// A startup target already visible from the top of the file does not
// scroll: the viewport stays at 0 and the no-scroll reveal leaves the
// saved vertical state untouched — none is recorded for the file.
func TestStartupRevealVisibleTargetStaysTop(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{longFile(5)})
	wide(t, m, idx)

	key := string(idx.Files[0].Path)
	if vp, ok := m.vps[key]; ok || vp.Top() != 0 {
		t.Fatalf("no-scroll startup reveal recorded state (ok=%v top=%d), want none", ok, vp.Top())
	}
	if row := frameRow(t, m, 1); !strings.Contains(row, "long line 1") {
		t.Fatalf("first content row = %q, want the file's first line", row)
	}
}

// Navigating to a hidden stop reveals it a third down; navigating back
// to a stop near BOF clamps to the top — available content takes
// precedence over one-third placement.
func TestNavigationRevealHiddenThenBOF(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{longFile(5, 200)})
	wide(t, m, idx)
	key := string(idx.Files[0].Path)

	m.Update(keyN) // line 200: row 199 → top 199 − 7
	if got := m.vps[key].Top(); got != 192 {
		t.Fatalf("n to line 200: top = %d, want 192", got)
	}
	if row := frameRow(t, m, 8); !strings.Contains(row, "long line 200") {
		t.Fatalf("frame row 8 = %q, want line 200 a third down", row)
	}

	m.Update(keyP) // line 5: row 4 − 7 clamps to 0
	if got := m.vps[key].Top(); got != 0 {
		t.Fatalf("p to line 5: top = %d, want the BOF clamp 0", got)
	}
	if row := frameRow(t, m, 5); !strings.Contains(row, "long line 5") {
		t.Fatalf("frame row 5 = %q, want line 5 near the top", row)
	}
}

// n between two already-on-screen stops moves only the current matched
// line — the viewport does not scroll.
func TestNavBetweenVisibleTargetsNoScroll(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{longFile(5, 10)})
	wide(t, m, idx)
	key := string(idx.Files[0].Path)

	m.Update(keyN) // line 10 is already visible from top 0
	if cur, _ := m.idx.Cursor(); cur.Stop != 1 {
		t.Fatalf("cursor stop = %d, want 1 (line 10)", cur.Stop)
	}
	if got := m.vps[key].Top(); got != 0 {
		t.Fatalf("n to a visible stop scrolled to top %d, want 0", got)
	}
	if row := frameRow(t, m, 1); !strings.Contains(row, "long line 1") {
		t.Fatalf("first content row = %q, want the unchanged top row", row)
	}
}

// A strict no-op — a one-stop index's n or p — is not a transition, so
// it triggers no reveal: the manually scrolled viewport stays put even
// with the only stop scrolled off screen.
func TestNoOpNavigationDoesNotReveal(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{
		{name: "solo.txt", content: numberedContent("solo", 60), stops: []navStop{{line: 1, start: 0, end: 4}}},
	})
	wide(t, m, idx)
	key := string(idx.Files[0].Path)

	for i := 0; i < 5; i++ {
		m.Update(keyDown)
	}
	m.Update(keyN)
	m.Update(keyP)
	if got := m.vps[key].Top(); got != 5 {
		t.Fatalf("no-op n/p revealed the off-screen stop: top = %d, want the scrolled 5", got)
	}
}

// A moving reveal replaces the file's saved vertical state: the
// revealed position — not the pre-navigation scroll — is what a later
// revisit resumes from.
func TestMovingRevealReplacesSavedViewport(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: numberedContent("a", 100), stops: []navStop{
			{line: 5, start: 0, end: 1},
			{line: 60, start: 0, end: 1},
		}},
		{name: "b.txt", content: numberedContent("b", 30), stops: []navStop{
			{line: 3, start: 0, end: 1},
		}},
	})
	wide(t, m, idx)
	keyA := string(idx.Files[0].Path)

	for i := 0; i < 10; i++ {
		m.Update(keyDown)
	}
	if got := m.vps[keyA].Top(); got != 10 {
		t.Fatalf("scrolled top = %d, want 10", got)
	}

	m.Update(keyN) // line 60: row 59 hidden → top 52, replacing the saved 10
	if got := m.vps[keyA].Top(); got != 52 {
		t.Fatalf("reveal to line 60: top = %d, want 52", got)
	}

	finishLoad(t, m, navCmd(t, m, keyN)) // to b.txt
	navSendsNoLoad(t, m, keyP)           // back to cached a.txt — pop-up only
	// The revisit starts from the replaced viewport: row 59 is visible
	// from top 52, so the no-scroll reveal keeps it.
	if got := m.vps[keyA].Top(); got != 52 {
		t.Fatalf("revisit top = %d, want the reveal-saved 52", got)
	}
	if row := frameRow(t, m, 1); !strings.Contains(row, "a line 53") {
		t.Fatalf("first content row = %q, want line 53 at top 52", row)
	}
}

// A revisit starts from the saved per-file viewport, not the top of
// the file: when the destination's target row is visible from the
// saved position the reveal does not scroll.
func TestRevisitRevealStartsFromSavedViewport(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{
			{line: 5, start: 0, end: 1},
			{line: 20, start: 0, end: 1},
		}},
		{name: "b.txt", content: numberedContent("b", 30), stops: []navStop{
			{line: 3, start: 0, end: 1},
		}},
	})
	wide(t, m, idx)
	keyA := string(idx.Files[0].Path)

	for i := 0; i < 10; i++ {
		m.Update(keyDown) // saved top 10; line 20's row 19 stays visible
	}
	m.Update(keyN) // same-file n to line 20: visible → no scroll
	if got := m.vps[keyA].Top(); got != 10 {
		t.Fatalf("n to visible line 20 scrolled to %d, want the saved 10", got)
	}

	finishLoad(t, m, navCmd(t, m, keyN)) // to b.txt, first visit
	navSendsNoLoad(t, m, keyP)           // back to cached a.txt — pop-up only
	// Back at a.txt's line-20 stop: starting from the saved top 10 the
	// target row 19 is still visible, so the viewport stays — a
	// top-of-file start would have left it at 0.
	if got := m.vps[keyA].Top(); got != 10 {
		t.Fatalf("revisit top = %d, want the saved 10", got)
	}
	if row := frameRow(t, m, 1); !strings.Contains(row, "a line 11") {
		t.Fatalf("first content row = %q, want line 11 at top 10", row)
	}
}

// A first visit — including a file whose buffer arrived while it was
// not current — starts from the top of the file before the reveal: a
// target hidden from top 0 lands at floor(h/3).
func TestFirstVisitStartsFromTopThenReveal(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, popupStubTicks)
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{{line: 1, start: 0, end: 1}}},
		{name: "b.txt", content: numberedContent("b", 60), stops: []navStop{{line: 40, start: 0, end: 1}}},
	})
	wide(t, m, idx)

	// b.txt's buffer lands while it is not current: it is cached but
	// never visited, so no reveal has been applied for it. Its request
	// was minted as an earlier entry would have minted it.
	buf, err := filebuffer.Load(idx.Files[1].Path, idx.Files[1].Stops)
	if err != nil {
		t.Fatal(err)
	}
	mintRequest(m, idx.Files[1].Path)
	injectLoad(t, m, fileLoadedMsg{path: idx.Files[1].Path, buf: buf})

	// The crossing opens a pop-up, but the cached file issues no load.
	for _, msg := range navLeafMsgs(t, m, keyN) {
		if _, ok := msg.(fileLoadedMsg); ok {
			t.Fatalf("n to a cached file issued a load: %#v", msg)
		}
	}
	keyB := string(idx.Files[1].Path)
	if got := m.vps[keyB].Top(); got != 32 {
		t.Fatalf("first-visit reveal top = %d, want 32 (row 39 at content row 7)", got)
	}
	if row := frameRow(t, m, 8); !strings.Contains(row, "b line 40") {
		t.Fatalf("frame row 8 = %q, want the target line a third down", row)
	}
}

// Navigating to an uncached file issues its load; when the load
// completes for the now-current file the same reveal sequence runs —
// first visit, so from the top of the file — landing the target a
// third down.
func TestNavToUncachedFileRevealsOnLoad(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := navIndex(t, []navFile{
		{name: "a.txt", content: numberedContent("a", 60), stops: []navStop{{line: 1, start: 0, end: 1}}},
		{name: "b.txt", content: numberedContent("b", 60), stops: []navStop{{line: 40, start: 0, end: 1}}},
	})
	wide(t, m, idx)

	finishLoad(t, m, navCmd(t, m, keyN)) // into b.txt
	keyB := string(idx.Files[1].Path)
	if got := m.vps[keyB].Top(); got != 32 {
		t.Fatalf("post-load reveal top = %d, want 32", got)
	}
	if row := frameRow(t, m, 8); !strings.Contains(row, "b line 40") {
		t.Fatalf("frame row 8 = %q, want the target line a third down", row)
	}
}
