package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clipperhouse/displaywidth"

	"vrg/internal/filebuffer"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// utf16le is the UTF-16 LE encoding of s under its BOM — the fixture
// bytes a BOM-marked file carries on disk.
func utf16le(s string) []byte {
	out := []byte{0xff, 0xfe}
	for _, r := range s {
		out = append(out, byte(r), 0x00)
	}
	return out
}

// writeEncodedFile creates a real file of exact bytes — BOM-marked
// content the string-typed writeMatchFile helper cannot express —
// creating any directories the fixture's path needs.
func writeEncodedFile(t *testing.T, dir, name string, content []byte) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

// unsupportedBuffer builds the prepared source a BOM-marked file
// produces — a real Load result for delivery through an injected
// loadResult or a stubLoader.
func unsupportedBuffer(t *testing.T, stops []searchindex.Stop) *filebuffer.Buffer {
	t.Helper()
	path := filepath.Join(t.TempDir(), "u16.bin")
	writeEncodedFile(t, filepath.Dir(path), filepath.Base(path), utf16le("hit\n"))
	buf, err := filebuffer.Load([]byte(path), stops)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if buf.Unsupported() == "" {
		t.Fatal("the UTF-16 fixture did not come out unsupported")
	}
	return buf
}

// The current file detected unsupported interrupts with the explanatory
// overlay while the panel shows "(unsupported encoding)": no file text,
// no highlights, no stale note — the filename row still names the path
// and the file's stops stay navigable.
func TestUnsupportedEncodingCurrentFile(t *testing.T) {
	dir := t.TempDir()
	writeEncodedFile(t, dir, "a.txt", utf16le("hit one\nhit two\n"))
	writeMatchFile(t, dir, "b.txt", "hit b\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit one\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("a.txt", "hit two\n", 2, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd()) // a.txt's load detects UTF-16 LE
	m.theme = theme.Plain()

	want := "cannot display a.txt: unsupported encoding UTF-16 LE"
	if m.overlay == nil {
		t.Fatal("the current file's unsupported encoding opened no overlay")
	}
	if len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("overlay lines = %q, want %q", m.overlay.lines, want)
	}
	if got := countDiag(m, want); got != 1 {
		t.Fatalf("collected %d occurrences of %q, want 1", got, want)
	}

	// Dismissal leaves the placeholder under a filename row still
	// naming the path — no file text, no stale note — and the stops
	// navigable.
	m, _ = update(t, m, keyPress("esc"))
	v := m.View().Content
	for _, want := range []string{"── a.txt ", "(unsupported encoding)"} {
		if !strings.Contains(v, want) {
			t.Fatalf("unsupported-file panel lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "hit") {
		t.Fatalf("the unsupported file rendered file text:\n%s", v)
	}
	if row := viewRow(t, m, 0); strings.Contains(row, "file changed since search") {
		t.Fatalf("stale validation ran on encoded bytes: %q", row)
	}
	m, c := update(t, m, keyMsg("n"))
	if c != nil {
		t.Fatalf("a same-file step on an unsupported file returned a command: %v", c)
	}
	stop, _ := m.index.Current()
	if stop.Line != 2 {
		t.Fatalf("n on an unsupported file selected %+v, want its second stop at line 2", stop)
	}
	if got := m.View().Content; !strings.Contains(got, "(unsupported encoding)") {
		t.Fatalf("the unsupported file lost its placeholder:\n%s", got)
	}
}

// A non-current file detected unsupported is collected as a diagnostic
// only — no overlay, no in-UI indicator, an untouched view — and is
// discovered by visiting the file, which shows the explanatory overlay
// from the cached detection without minting another load.
func TestUnsupportedEncodingNonCurrentDiagnosticOnly(t *testing.T) {
	dir := t.TempDir()
	writeMatchFile(t, dir, "a.txt", "hit a\n")
	writeEncodedFile(t, dir, "b.txt", utf16le("hit b\n"))
	writeMatchFile(t, dir, "c.txt", "hit c\n")
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit a\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("b.txt", "hit b\n", 1, 0, 3, "hit"))
	addRec(t, idx, matchRec("c.txt", "hit c\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd()) // a.txt loads
	m.theme = theme.Plain()

	// b.txt's load is requested on entry; hold its result undelivered —
	// indistinguishable to the model from a worker still running — and
	// move on to c.txt so the detection lands while b.txt is not
	// current.
	m, cmd = update(t, m, keyMsg("n"))
	msgB := navLoadMsg(t, cmd)
	m, cmd = update(t, m, keyMsg("n"))
	m = applyLoad(t, m, deliverNavLoad(t, cmd)) // c.txt loads
	before := m.View().Content

	m, _ = update(t, m, msgB)
	if m.overlay != nil {
		t.Fatal("a non-current unsupported detection opened the overlay")
	}
	if got := m.View().Content; got != before {
		t.Fatalf("a non-current detection changed the view:\nbefore:\n%s\nafter:\n%s", before, got)
	}
	want := "cannot display b.txt: unsupported encoding UTF-16 LE"
	if got := countDiag(m, want); got != 1 {
		t.Fatalf("collected %d occurrences of %q, want 1", got, want)
	}

	// Discovery on visit: p onto b.txt shows the explanatory overlay —
	// from the cached buffer, so no second load is minted.
	m, cmd = update(t, m, keyMsg("p"))
	if m.overlay == nil {
		t.Fatal("visiting the unsupported file opened no overlay")
	}
	if len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("visit overlay lines = %q, want the encoding diagnostic %q", m.overlay.lines, want)
	}
	for _, msg := range navMsgs(t, cmd) {
		if _, isLoad := msg.(loadResult); isLoad {
			t.Fatal("re-entering a detected unsupported file minted another load")
		}
	}
	if _, flying := m.loading["b.txt"]; flying {
		t.Fatal("re-entering a detected unsupported file left a load in flight")
	}
	m, _ = update(t, m, keyPress("esc"))
	if v := m.View().Content; !strings.Contains(v, "(unsupported encoding)") {
		t.Fatalf("unsupported panel lacks the placeholder:\n%s", v)
	}
}

// r on an unsupported file is the reload route: it reissues the load —
// "Loading…" under the filename row while in flight — and an unchanged
// file settles back to the placeholder with the explanatory overlay
// again. An open overlay blocks r; a file rewritten in a supported
// encoding loads as ordinary content.
func TestUnsupportedEncodingReload(t *testing.T) {
	dir := t.TempDir()
	writeEncodedFile(t, dir, "a.txt", utf16le("hit\n"))
	idx := searchindex.New(dir)
	addRec(t, idx, matchRec("a.txt", "hit\n", 1, 0, 3, "hit"))
	idx.Finish()

	m, cmd := startBrowse(t, dir, idx, 80, 24)
	m = applyLoad(t, m, cmd())
	m.theme = theme.Plain()
	want := "cannot display a.txt: unsupported encoding UTF-16 LE"

	// The open overlay blocks r: the key is an overlay input, not a
	// reload request.
	m, c := update(t, m, keyMsg("r"))
	if c != nil {
		t.Fatalf("r with the overlay open returned a command: %v", c)
	}
	if _, flying := m.loading["a.txt"]; flying {
		t.Fatal("r with the overlay open minted a load")
	}
	m, _ = update(t, m, keyPress("esc"))

	// r reissues the load: a fresh request is minted and the panel
	// reads "Loading…" under the filename row.
	m, cmd = update(t, m, keyMsg("r"))
	if cmd == nil {
		t.Fatal("r on an unsupported file issued no reload")
	}
	if _, flying := m.loading["a.txt"]; !flying {
		t.Fatal("r minted no in-flight request for a.txt")
	}
	if v := m.View().Content; !strings.Contains(v, "── a.txt ") ||
		!strings.Contains(v, "Loading…") || strings.Contains(v, "(unsupported encoding)") {
		t.Fatalf("reload panel lacks the filename row or the loading placeholder:\n%s", v)
	}
	msg := cmd()
	lr, ok := msg.(loadResult)
	if !ok || string(lr.path) != "a.txt" {
		t.Fatalf("the reload produced %#v, want the a.txt load", msg)
	}

	// Unchanged content settles back to the placeholder and explains
	// itself again — a new overlay, a second collected diagnostic.
	m = applyLoad(t, m, msg)
	if m.overlay == nil || len(m.overlay.lines) != 1 || m.overlay.lines[0] != want {
		t.Fatalf("the unchanged reload's overlay = %v, want %q", m.overlay, want)
	}
	if got := countDiag(m, want); got != 2 {
		t.Fatalf("collected %d occurrences of %q, want 2", got, want)
	}
	m, _ = update(t, m, keyPress("esc"))
	if v := m.View().Content; !strings.Contains(v, "(unsupported encoding)") {
		t.Fatalf("the unchanged reload lost the placeholder:\n%s", v)
	}

	// A file rewritten in a supported encoding reloads as ordinary
	// content: the placeholder and its status are gone.
	writeMatchFile(t, dir, "a.txt", "hit\n")
	m, cmd = update(t, m, keyMsg("r"))
	m = applyLoad(t, m, cmd())
	if v := m.View().Content; !strings.Contains(v, "hit") ||
		strings.Contains(v, "(unsupported encoding)") {
		t.Fatalf("a supported reload left the placeholder:\n%s", v)
	}
}

// The unsupported file's composed frame stays well-formed at ordinary
// and constrained widths: the filename row keeps identifying the
// truncated safe path through Issue 24's slot rules, the panel shows
// the placeholder, no row overflows the terminal, and every layout
// dimension stays nonnegative.
func TestUnsupportedEncodingComposedView(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("deep/", 10) + "end.txt" // 55 cells
	writeEncodedFile(t, dir, long, utf16le("hit\n"))
	writeMatchFile(t, dir, "z.txt", "hit\n")

	for _, size := range []struct{ w, h int }{{80, 24}, {30, 8}, {20, 3}} {
		idx := searchindex.New(dir)
		addRec(t, idx, matchRec(long, "hit\n", 1, 0, 3, "hit"))
		addRec(t, idx, matchRec("z.txt", "hit\n", 1, 0, 3, "hit"))
		idx.Finish()

		m, cmd := startBrowse(t, dir, idx, size.w, size.h)
		m = applyLoad(t, m, cmd()) // the long-named file detects while current
		m.theme = theme.Plain()
		m, _ = update(t, m, keyPress("esc"))

		v := m.View().Content
		for i, r := range strings.Split(v, "\n") {
			if got := displaywidth.String(r); got > size.w {
				t.Fatalf("row %d at %dx%d is %d cells wide, overflowing:\n%q",
					i, size.w, size.h, got, r)
			}
		}
		if !strings.Contains(v, "(unsupported") {
			t.Fatalf("%dx%d panel lacks the placeholder:\n%s", size.w, size.h, v)
		}
		if !strings.Contains(v, "…") || !strings.Contains(v, "end.txt") {
			t.Fatalf("%dx%d filename row lost the truncated safe path:\n%s", size.w, size.h, v)
		}
		if m.listWidth(size.w) < 0 || m.panelWidth() < 0 || m.contentHeight() < 0 {
			t.Fatalf("%dx%d produced a negative layout dimension", size.w, size.h)
		}
	}
}
