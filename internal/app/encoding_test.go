package app

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// u16le is the UTF-16 LE fixture's bytes: the BOM followed by "hi\n"
// in two-byte little-endian code units.
const u16le = "\xff\xfeh\x00i\x00\n\x00"

// The current file detecting an unsupported BOM interrupts like a
// read failure: the explanatory overlay opens over the
// "(unsupported encoding)" placeholder, no file text or highlight
// paints, the filename row still names the path, and one diagnostic
// is collected for exit-time replay.
func TestUnsupportedCurrentFileNotifies(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "u16.txt", u16le)
	recs := append(fileRecs("u16.txt",
		matchLineRec("u16.txt", "hi", 1, "hi", 0, 2),
		matchLineRec("u16.txt", "hi", 2, "hi", 0, 2)), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	if m.overlay == nil {
		t.Fatal("an unsupported current file did not open the explanatory overlay")
	}
	v := ansi.Strip(m.View().Content)
	if !strings.Contains(v, "unsupported encoding UTF-16 LE") || !strings.Contains(v, "u16.txt") {
		t.Fatalf("overlay lacks the explanatory diagnostic: %q", v)
	}
	if len(m.diags) != 1 || !strings.Contains(m.diags[0], "unsupported encoding") {
		t.Fatalf("diags = %v, want the one collected encoding diagnostic", m.diags)
	}

	m, _ = pressKey(t, m, "esc")
	v = ansi.Strip(m.View().Content)
	if !strings.Contains(v, "(unsupported encoding)") {
		t.Fatalf("panel lacks the placeholder: %q", v)
	}
	if strings.Contains(v, "^@") {
		t.Fatalf("encoded file bytes leaked into the panel: %q", v)
	}
	if strings.Contains(m.View().Content, "\x1b[30;47") {
		t.Fatal("a placeholder painted an inverse search highlight")
	}
	if !strings.Contains(v, "u16.txt") {
		t.Fatalf("the filename row stopped naming the path: %q", v)
	}
}

// An unsupported file stays an indexed cursor stop: n/p traverse its
// stops like any file's, and re-entering it re-opens the explanatory
// overlay — the Issue #26 re-entry route applied to encodings —
// rather than the file-change pop-up.
func TestUnsupportedFileRemainsACursorStop(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hi there\n")
	writeWorkFile(t, dir, "u16.txt", u16le)
	recs := append(fileRecs("a.txt", matchLineRec("a.txt", "hi there", 1, "hi", 0, 2)),
		fileRecs("u16.txt",
			matchLineRec("u16.txt", "hi", 1, "hi", 0, 2),
			matchLineRec("u16.txt", "hi", 2, "hi", 0, 2))...)
	recs = append(recs, recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	m, cmd = update(t, m, keyPress("n"))
	m = settle(t, m, cmd)
	if m.overlay == nil {
		t.Fatal("entering the unsupported file did not open its overlay")
	}
	if s, _ := m.currentStop(); string(s.Path) != "u16.txt" || s.Line != 1 {
		t.Fatalf("cursor stop = %s:%d, want u16.txt:1", s.Path, s.Line)
	}
	m, _ = pressKey(t, m, "esc")

	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); string(s.Path) != "u16.txt" || s.Line != 2 {
		t.Fatalf("n within the unsupported file: stop = %s:%d, want u16.txt:2", s.Path, s.Line)
	}
	m, _ = update(t, m, keyPress("n"))
	if s, _ := m.currentStop(); string(s.Path) != "a.txt" || s.Line != 1 {
		t.Fatalf("n wrapping off the unsupported file: stop = %s:%d, want a.txt:1", s.Path, s.Line)
	}

	m, _ = update(t, m, keyPress("n"))
	if m.overlay == nil {
		t.Fatal("re-entering the unsupported file did not re-open its overlay")
	}
	if len(m.diags) != 1 {
		t.Fatalf("diags = %v, want the single collection retained", m.diags)
	}
}

// Detection on a non-current path is diagnostic-only — one collection,
// no overlay, no painted indicator — and visiting the file then
// surfaces the explanation through the same overlay the current path
// would have shown.
func TestUnsupportedNonCurrentIsDiagnosticOnly(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "a.txt", "hi there\n")
	writeWorkFile(t, dir, "u16.txt", u16le)
	recs := append(fileRecs("a.txt", matchLineRec("a.txt", "hi there", 1, "hi", 0, 2)),
		fileRecs("u16.txt", matchLineRec("u16.txt", "hi", 1, "hi", 0, 2))...)
	recs = append(recs, recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	m, cmd = update(t, m, keyPress("n"))
	m, _ = update(t, m, keyPress("p"))
	before := m.View().Content
	diags := len(m.diags)
	m = settle(t, m, cmd)
	if m.overlay != nil {
		t.Fatal("a non-current detection opened an overlay")
	}
	if got := m.View().Content; got != before {
		t.Fatalf("the non-current completion repainted the frame:\n%s\n%s", before, got)
	}
	if len(m.diags) != diags+1 || !strings.Contains(m.diags[diags], "unsupported encoding") {
		t.Fatalf("diags = %v, want the one collected encoding diagnostic", m.diags)
	}

	m, _ = update(t, m, keyPress("n"))
	if m.overlay == nil {
		t.Fatal("visiting the unsupported file did not surface its overlay")
	}
	v := ansi.Strip(m.View().Content)
	if !strings.Contains(v, "unsupported encoding UTF-16 LE") {
		t.Fatalf("overlay lacks the explanatory diagnostic: %q", v)
	}
	if got := contentRow1(t, m); !strings.Contains(got, "(unsupported encoding)") {
		t.Fatalf("content row = %q, want the placeholder", got)
	}
	if len(m.diags) != diags+1 {
		t.Fatal("the visit re-collected the diagnostic")
	}
}

// r on an unsupported file is the Issue #27 reload route: the open
// overlay swallows r while it is up; dismissed, r drops the
// placeholder to "Loading…", issues exactly one reread, and the
// still-UTF-16 completion restores the placeholder with a fresh
// overlay and a second collected diagnostic.
func TestUnsupportedReloadReissuesAndPreserves(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "u16.txt", u16le)
	recs := append(fileRecs("u16.txt", matchLineRec("u16.txt", "hi", 1, "hi", 0, 2)), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	m2, c := update(t, m, keyPress("r"))
	if c != nil {
		t.Fatal("r under the open overlay issued a command")
	}
	if m2.loading["u16.txt"] != 0 {
		t.Fatal("r under the open overlay started a reload")
	}
	m, _ = pressKey(t, m, "esc")

	m, reload := update(t, m, keyPress("r"))
	if reload == nil {
		t.Fatal("r on the unsupported file issued no reload")
	}
	if got := contentRow1(t, m); !strings.Contains(got, "Loading…") {
		t.Fatalf("content row = %q, want the Loading… placeholder", got)
	}
	var loads []tea.Msg
	for _, msg := range cmdMsgs(reload) {
		if ld, ok := msg.(loadDoneMsg); ok {
			if string(ld.path) != "u16.txt" {
				t.Fatalf("reload read %q, want u16.txt", ld.path)
			}
			loads = append(loads, msg)
		}
	}
	if len(loads) != 1 {
		t.Fatalf("r produced %d load requests, want 1", len(loads))
	}

	diags := len(m.diags)
	m = pump(t, m, loads[0])
	if m.overlay == nil {
		t.Fatal("the reload's detection did not re-open the overlay")
	}
	if len(m.diags) != diags+1 {
		t.Fatal("the re-detected encoding was not collected")
	}
	m, _ = pressKey(t, m, "esc")
	if got := contentRow1(t, m); !strings.Contains(got, "(unsupported encoding)") {
		t.Fatalf("content row = %q, want the restored placeholder", got)
	}
}

// Unsupported bytes never produce the Issue #29 stale verdict: the
// buffer skips raw-byte validation, so no "file changed since search"
// note can appear and no inverse highlight paints — even though rg's
// transcoded submatch offsets can never match the raw encoded bytes.
func TestUnsupportedFileIsNeverStale(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "u16.txt", u16le)
	recs := append(fileRecs("u16.txt", matchLineRec("u16.txt", "hi", 1, "hi", 0, 2)), recSummary)
	m, cmd := browseModel(t, dir, 80, 24, recs...)
	m = settle(t, m, cmd)

	if m.stale["u16.txt"] {
		t.Fatal("the unsupported file was marked stale")
	}
	if m.bufs["u16.txt"] == nil || m.bufs["u16.txt"].Unsupported() == "" {
		t.Fatal("no unsupported buffer was cached")
	}
	m, _ = pressKey(t, m, "esc")
	if v := ansi.Strip(m.View().Content); strings.Contains(v, staleNote) {
		t.Fatalf("the stale note leaked onto an unsupported file: %q", v)
	}
}

// The unsupported state composes at every size: the filename row
// keeps identifying the truncated safe path, the panel carries the
// placeholder, nothing overflows, and dimensions stay nonnegative —
// the Issue #26 composed-view contract applied to the placeholder.
func TestUnsupportedComposedViewStaysWellFormed(t *testing.T) {
	dir := t.TempDir()
	name := "a-very-long-filename-with\ttabs\nand-newlines-and-a-long-tail.txt"
	writeWorkFile(t, dir, name, u16le)
	q := strconv.Quote(name)
	recs := []string{
		fmt.Sprintf(`{"type":"begin","data":{"path":{"text":%s}}}`, q),
		fmt.Sprintf(`{"type":"match","data":{"path":{"text":%s},"lines":{"text":"hi\n"},"line_number":1,"submatches":[{"match":{"text":"hi"},"start":0,"end":2}]}}`, q),
		fmt.Sprintf(`{"type":"end","data":{"path":{"text":%s},"binary_offset":null}}`, q),
		recSummary,
	}
	for _, sz := range []struct{ w, h int }{{80, 24}, {40, 10}, {26, 6}, {20, 3}} {
		m, cmd := browseModel(t, dir, sz.w, sz.h, recs...)
		m = settle(t, m, cmd)
		m, _ = pressKey(t, m, "esc")
		v := ansi.Strip(m.View().Content)
		rows := strings.Split(v, "\n")
		if len(rows) != sz.h {
			t.Fatalf("%dx%d: frame has %d rows", sz.w, sz.h, len(rows))
		}
		for i, row := range rows {
			if got := ansi.StringWidth(row); got > sz.w {
				t.Fatalf("%dx%d row %d is %d cells: %q", sz.w, sz.h, i, got, row)
			}
		}
		if m.listW < 0 || m.textW < 0 {
			t.Fatalf("%dx%d: listW=%d textW=%d", sz.w, sz.h, m.listW, m.textW)
		}
		if !strings.Contains(rows[0], "…") {
			t.Fatalf("%dx%d: filename row lost the truncation marker: %q", sz.w, sz.h, rows[0])
		}
		if sz.w >= 80 && !strings.Contains(rows[0], "tail.txt") {
			t.Fatalf("filename row lost the safe-path tail: %q", rows[0])
		}
		if !strings.Contains(v, "unsupport") {
			t.Fatalf("%dx%d: the placeholder is missing: %q", sz.w, sz.h, v)
		}
	}
}
