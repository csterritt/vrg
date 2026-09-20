package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// fakeChild is a Child whose piped streams are preloaded; Wait records
// that the child was reaped so tests can prove rg had exited.
type fakeChild struct {
	stdout  io.Reader
	stderr  io.Reader
	waitErr error
	waited  chan struct{}
	once    sync.Once
}

func (f *fakeChild) Stdout() io.Reader { return f.stdout }
func (f *fakeChild) Stderr() io.Reader { return f.stderr }
func (f *fakeChild) Wait() error {
	f.once.Do(func() {
		if f.waited != nil {
			close(f.waited)
		}
	})
	return f.waitErr
}

const validStream = `{"type":"begin","data":{"path":{"text":"a.txt"}}}` + "\n" +
	`{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit\n"},` +
	`"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}` + "\n" +
	`{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}` + "\n" +
	`{"type":"summary","data":{"stats":{}}}` + "\n"

// fixtureIndex builds a finished index with the given number of files,
// each with the given number of matched lines.
func fixtureIndex(t *testing.T, files, linesPerFile int) *searchindex.Index {
	t.Helper()
	idx := searchindex.New("/w")
	for f := 0; f < files; f++ {
		path := fmt.Sprintf("%c.txt", 'a'+f)
		for l := 1; l <= linesPerFile; l++ {
			rec := fmt.Sprintf(`{"type":"match","data":{"path":{"text":%q},`+
				`"lines":{"text":"hit\n"},"line_number":%d,`+
				`"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`, path, l)
			if err := idx.Add([]byte(rec)); err != nil {
				t.Fatalf("fixture Add: %v", err)
			}
		}
	}
	idx.Finish()
	return idx
}

func keyMsg(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: rune(s[0]), Text: s}
}

// The searching screen is shown while results are collected.
func TestSearchingScreenShown(t *testing.T) {
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	if got := m.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("searching View = %q, want it to contain %q", got, "Searching…")
	}
}

// "Searching…" covers collection and post-exit processing: with index
// preparation held by a gate, the app stays in the searching state even
// after rg has exited.
func TestSearchingCoversPostExitPreparation(t *testing.T) {
	child := &fakeChild{
		stdout: strings.NewReader(validStream),
		stderr: strings.NewReader(""),
		waited: make(chan struct{}),
	}
	m := New(child, "/w")
	gate := make(chan struct{})
	m.gate = gate

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init() returned no collection command")
	}
	msgs := make(chan tea.Msg, 1)
	go func() { msgs <- cmd() }()

	<-child.waited // the child has exited; preparation is still gated.

	select {
	case <-msgs:
		t.Fatal("collection delivered its result while index preparation was gated")
	default:
	}
	if got := m.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("View after child exit = %q, want it to still show %q", got, "Searching…")
	}

	close(gate)
	var msg tea.Msg
	select {
	case msg = <-msgs:
	case <-time.After(10 * time.Second):
		t.Fatal("collection did not deliver after the gate was released")
	}
	m2, _ := m.Update(msg)
	if got := m2.(Model).View().Content; !strings.Contains(got, "1 files, 1 matched lines") {
		t.Fatalf("summary View = %q, want %q", got, "1 files, 1 matched lines")
	}
}

// An injected search-completion message transitions the model to the
// interim summary screen: "N files, M matched lines".
func TestCompletionTransitionsToSummary(t *testing.T) {
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m2, _ := m.Update(searchResult{index: fixtureIndex(t, 2, 3)})
	got := m2.(Model).View().Content
	if !strings.Contains(got, "2 files, 6 matched lines") {
		t.Fatalf("summary View = %q, want %q", got, "2 files, 6 matched lines")
	}
	if strings.Contains(got, "Searching") {
		t.Fatalf("summary View still shows searching: %q", got)
	}
}

// q on the interim summary quits with exit status 0.
func TestSummaryQuitExitsZero(t *testing.T) {
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m2, _ := m.Update(searchResult{index: fixtureIndex(t, 1, 1)})
	m3, cmd := m2.(Model).Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("q on the summary returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q on the summary returned %T, want tea.QuitMsg", cmd())
	}
	if s := m3.(Model).status; s != 0 {
		t.Fatalf("exit status = %d, want 0", s)
	}
}

// A resize while searching is handled without blocking on collection.
func TestResizeWhileSearching(t *testing.T) {
	m := New(&fakeChild{stdout: strings.NewReader(""), stderr: strings.NewReader("")}, "/w")
	m2, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if cmd != nil {
		t.Fatalf("resize returned a command: %v", cmd)
	}
	mm := m2.(Model)
	if mm.width != 100 || mm.height != 40 {
		t.Fatalf("size = %dx%d, want 100x40", mm.width, mm.height)
	}
	if got := mm.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("View after resize = %q, want it to still show %q", got, "Searching…")
	}
}

// A child that cannot start produces a sanitized stderr diagnostic and
// exit 2 without the TUI ever running.
func TestStartFailureExitsTwoWithoutTUI(t *testing.T) {
	var stderr bytes.Buffer
	programRan := false
	code := Run([]string{"--json", "--no-config", "--", "foo", "."}, Env{
		Start: func(argv []string, workdir string) (Child, error) {
			return nil, errors.New(`exec: "rg": executable file not found in $PATH`)
		},
		Stderr: &stderr,
		Program: func(Model) (tea.Model, error) {
			programRan = true
			return nil, nil
		},
	})
	if code != 2 {
		t.Fatalf("Run exit = %d, want 2", code)
	}
	if programRan {
		t.Fatal("the TUI program ran after a start failure")
	}
	diag := strings.TrimSuffix(stderr.String(), "\n")
	if !strings.HasPrefix(diag, "vrg: ") || !strings.Contains(diag, "cannot start rg") {
		t.Fatalf("stderr = %q, want a vrg: diagnostic naming the rg start failure", stderr.String())
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("start failure emitted usage help: %q", stderr.String())
	}
	if strings.ContainsAny(diag, "\x1b\x9b") || strings.Contains(diag, "\n") {
		t.Fatalf("diagnostic is not a sanitized single line: %q", stderr.String())
	}
}

// Hostile bytes in the start error are escaped, never emitted raw.
func TestStartFailureDiagnosticIsSanitized(t *testing.T) {
	var stderr bytes.Buffer
	code := Run([]string{"--json", "--no-config", "--", "foo", "."}, Env{
		Start: func(argv []string, workdir string) (Child, error) {
			return nil, errors.New("bad \x1b[31m error\nhere")
		},
		Stderr: &stderr,
	})
	if code != 2 {
		t.Fatalf("Run exit = %d, want 2", code)
	}
	if strings.Contains(stderr.String(), "\x1b") || strings.Count(stderr.String(), "\n") != 1 {
		t.Fatalf("hostile error reached stderr unsanitized: %q", stderr.String())
	}
}

// The production path through collect: a real started child's stream is
// indexed and both pipes are drained.
func TestCollectIndexesStream(t *testing.T) {
	child := &fakeChild{
		stdout: strings.NewReader(validStream),
		stderr: strings.NewReader("warning text\n"),
	}
	res := collect(context.Background(), child, "/w", nil)
	if res.err != nil {
		t.Fatalf("collect err = %v", res.err)
	}
	stops := res.index.Stops()
	if len(stops) != 1 || string(stops[0].Path) != "a.txt" || stops[0].Line != 1 {
		t.Fatalf("index stops = %+v, want a.txt:1", stops)
	}
	if string(res.stderr) != "warning text\n" {
		t.Fatalf("stderr = %q, want %q", res.stderr, "warning text\n")
	}
}
