package app

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/filebuffer"
	"vrg/internal/safepresentation"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// utf16LEContent is the UTF-16 LE disk content for "hi\n" — the bytes
// real files carry and real rg transcodes before matching (Issue #30).
const utf16LEContent = "\xff\xfeh\x00i\x00\n\x00"

// encFile is an unsupported-encoding fixture: raw is the on-disk byte
// content — BOM signature included — while line and match record the
// transcoded rg-line view ripgrep reports for it, whose bytes can
// never equal a range of the raw file.
type encFile struct {
	name       string
	raw        string // bytes on disk, BOM included
	line       string // the recorded rg line — the transcoded view
	match      string // the recorded submatch bytes
	start, end int
}

// encIndex writes the fixture files and builds the index with one
// match record per file carrying the recorded transcoded line — the
// form a real rg produces for BOM-marked UTF-16/32 files.
func encIndex(t *testing.T, files []encFile) *searchindex.Index {
	t.Helper()
	dir := t.TempDir()
	var stream strings.Builder
	b64 := func(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.raw), 0o644); err != nil {
			t.Fatal(err)
		}
		path := "./" + f.name
		fmt.Fprintf(&stream, `{"type":"begin","data":{"path":{"bytes":%q}}}`+"\n", b64(path))
		fmt.Fprintf(&stream, `{"type":"match","data":{"path":{"bytes":%q},"lines":{"bytes":%q},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"bytes":%q},"start":%d,"end":%d}]}}`+"\n",
			b64(path), b64(f.line), b64(f.match), f.start, f.end)
		fmt.Fprintf(&stream, `{"type":"end","data":{"path":{"bytes":%q},"binary_offset":null,"stats":{}}}`+"\n", b64(path))
	}
	stream.WriteString(`{"type":"summary","data":{"stats":{}}}` + "\n")
	return searchindex.Build([]byte(stream.String()), dir)
}

// encFiles is the two-file encoding fixture: a.txt is ordinary text,
// u16.txt is UTF-16 LE — one recorded stop each so n crosses between
// them.
var encFiles = []encFile{
	{name: "a.txt", raw: "alpha one\n", line: "alpha one\n", match: "alpha", start: 0, end: 5},
	{name: "u16.txt", raw: utf16LEContent, line: "hi\n", match: "hi", start: 0, end: 2},
}

// Entering a file whose BOM marks UTF-16 shows "(unsupported
// encoding)" with no file text and no highlights, opens the overlay
// with the explanatory diagnostic, and keeps the file an indexed
// cursor stop (Issue #30's current-file notification).
func TestUnsupportedCurrentShowsOverlayAndPlaceholder(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{popupTimer: popupStubTicks.popupTimer})
	idx := encIndex(t, encFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	keyU := string(idx.Files[1].Path)

	msg, ok := fileLoadOf(navCmd(t, m, keyN))
	if !ok {
		t.Fatal("crossing into u16.txt produced no load completion")
	}
	m.Update(msg)

	if got := m.bufs[keyU].Unsupported(); got != "UTF-16 LE" {
		t.Fatalf("the buffer's encoding = %q, want UTF-16 LE", got)
	}
	if !m.overlayOpen {
		t.Fatal("a current-file encoding detection did not open the overlay")
	}
	want := encodingDiag(idx.Files[1].Path, "UTF-16 LE")
	if !strings.Contains(m.overlayText, want) {
		t.Fatalf("overlay = %q, want the encoding diagnostic %q", m.overlayText, want)
	}
	v := viewText(m)
	if !strings.Contains(v, "(unsupported encoding)") {
		t.Fatalf("view = %q, want the (unsupported encoding) placeholder", v)
	}
	if !strings.Contains(v, "─ "+escapedPath(idx.Files[1])+" ") {
		t.Fatalf("view = %q, want the filename row still naming the path", v)
	}
	if strings.Contains(v, "^@") || strings.Contains(v, "\x1b[30;47") {
		t.Fatalf("view = %q, want no file text and no highlights", v)
	}
	assertReplayLines(t, m.diags, []string{want})

	// The file stays an indexed cursor stop: n wraps back to a.txt,
	// minting no load for either cached file — only the pop-up.
	m.Update(keyEsc)
	navSendsNoLoad(t, m, keyN)
	if cur, _ := m.idx.Cursor(); cur.File != 0 {
		t.Fatalf("n from the unsupported file selected %+v, want the wrapped a.txt stop", cur)
	}
}

// A detected encoding for a non-current file is collected as a
// diagnostic only: no overlay and no frame change — the Issue #26
// current/non-current distinction. The detection surfaces when the
// user visits the file or in the replay.
func TestUnsupportedNonCurrentDiagnosticOnly(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{popupTimer: popupStubTicks.popupTimer})
	idx := encIndex(t, encFiles)
	finishLoad(t, m, startBrowse(t, m, idx))
	before := viewText(m)

	m.Update(fileLoadedMsg{
		path: idx.Files[1].Path,
		req:  mintRequest(m, idx.Files[1].Path),
		buf:  filebuffer.Decode([]byte(utf16LEContent), idx.Files[1].Stops),
	})
	if m.overlayOpen {
		t.Fatal("a non-current encoding detection opened the overlay")
	}
	if got := viewText(m); got != before {
		t.Fatalf("a non-current detection changed the frame:\nbefore %q\nafter  %q", before, got)
	}
	want := encodingDiag(idx.Files[1].Path, "UTF-16 LE")
	assertReplayLines(t, m.diags, []string{want})

	// Visiting the file later shows the placeholder from the cached
	// buffer — no load is minted — and collects nothing new.
	navSendsNoLoad(t, m, keyN)
	if v := viewText(m); !strings.Contains(v, "(unsupported encoding)") {
		t.Fatalf("view = %q, want the (unsupported encoding) placeholder on the visit", v)
	}
	assertReplayLines(t, m.diags, []string{want})
}

// r on an unsupported file reissues the load — "Loading…" while it is
// in flight — and an unchanged file settles back to the placeholder
// with a fresh overlay and a second collected diagnostic (Issue #30's
// reloadable placeholder).
func TestUnsupportedReloadPreservesPlaceholder(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := encIndex(t, []encFile{
		{name: "u16.txt", raw: utf16LEContent, line: "hi\n", match: "hi", start: 0, end: 2},
	})
	finishLoad(t, m, startBrowse(t, m, idx))
	keyU := string(idx.Files[0].Path)
	want := encodingDiag(idx.Files[0].Path, "UTF-16 LE")
	assertReplayLines(t, m.diags, []string{want})
	m.Update(keyEsc) // dismiss the first detection's overlay

	_, cmd := m.Update(keyR)
	if cmd == nil {
		t.Fatal("r on the unsupported file minted no reload")
	}
	if _, ok := m.loading[keyU]; !ok {
		t.Fatal("r recorded no in-flight request for the unsupported file")
	}
	if v := viewText(m); !strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want Loading… during the reload", v)
	}
	msg, ok := fileLoadOf(cmd)
	if !ok {
		t.Fatal("the reload produced no completion")
	}
	m.Update(msg)

	if v := viewText(m); !strings.Contains(v, "(unsupported encoding)") {
		t.Fatalf("view = %q, want the placeholder again after the unchanged reload", v)
	}
	if !m.overlayOpen || !strings.Contains(m.overlayText, want) {
		t.Fatalf("overlay open=%v text=%q, want the reissued detection shown",
			m.overlayOpen, m.overlayText)
	}
	assertReplayLines(t, m.diags, []string{want, want})
}

// Stale validation never runs on the encoded bytes (Issue #30): the
// recorded submatch — rg's transcoded "hi" — cannot byte-equal any
// raw range, so the Issue #29 check would mark the file stale and set
// the note. The buffer stays clean and the filename row carries none.
func TestUnsupportedRunsNoStaleValidation(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	idx := encIndex(t, []encFile{
		{name: "u16.txt", raw: utf16LEContent, line: "hi\n", match: "hi", start: 0, end: 2},
	})
	finishLoad(t, m, startBrowse(t, m, idx))

	if m.bufs[string(idx.Files[0].Path)].Stale() {
		t.Fatal("stale validation ran against the unsupported file's raw bytes")
	}
	m.Update(keyEsc)
	if row := frameRow(t, m, 0); strings.Contains(row, staleNote) {
		t.Fatalf("filename row = %q, want no %q — validation never ran", row, staleNote)
	}
}

// The unsupported state composes safely at ordinary and constrained
// widths: the filename row keeps the left-truncated safe path in its
// Issue #24 slot, the panel shows the placeholder, no row overflows
// the frame, and every layout dimension stays nonnegative.
func TestUnsupportedComposedViewAtConstrainedWidths(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Code: 0}}, options{})
	m.theme = theme.Plain()
	idx := encIndex(t, []encFile{{
		name:  "deep-" + strings.Repeat("n", 90) + "-leaf.txt",
		raw:   utf16LEContent,
		line:  "hi\n",
		match: "hi",
		start: 0, end: 2,
	}})
	msg, ok := fileLoadOf(startBrowse(t, m, idx))
	if !ok {
		t.Fatal("the startup load produced no completion")
	}
	m.Update(msg)
	m.Update(keyEsc) // dismiss the detection overlay to see the frame

	for _, w := range []int{80, 30, 20} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 24})
		v := viewText(m)
		if got := strings.Count(v, "\n") + 1; got != 24 {
			t.Fatalf("width %d: view rows = %d, want 24", w, got)
		}
		// The placeholder clips to the panel's cells at constrained
		// widths — the unclipped prefix still identifies it.
		want := "(unsupported"
		if w == 80 {
			want = "(unsupported encoding)"
		}
		if !strings.Contains(v, want) {
			t.Fatalf("width %d: view = %q, want the placeholder %q", w, v, want)
		}
		rule := frameRow(t, m, 0)
		if !strings.Contains(rule, "…") || !strings.Contains(rule, "leaf.txt") {
			t.Fatalf("width %d: filename row = %q, want the …-truncated path ending at the basename", w, rule)
		}
		for r := 0; r < 24; r++ {
			if cw := safepresentation.CellWidth(frameRow(t, m, r)); cw > w {
				t.Fatalf("width %d: row %d is %d cells — overflow: %q", w, r, cw, frameRow(t, m, r))
			}
		}
		if m.listWidth() < 0 || w-m.listWidth() < 0 {
			t.Fatalf("width %d: negative layout dimension: list %d", w, m.listWidth())
		}
	}
}
