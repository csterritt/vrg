package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/searchindex"
)

// emptyStream is a complete zero-result ripgrep JSON stream — summary
// only — as an rg-1 exit produces.
const emptyStream = `{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// allBinaryStream is a complete stream in which every matched file's
// end event reports a non-null binary_offset: all results are excluded,
// leaving no usable stops.
const allBinaryStream = `{"type":"begin","data":{"path":{"text":"./a.bin"}}}
{"type":"match","data":{"path":{"text":"./a.bin"},"lines":{"text":"foo one\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}
{"type":"match","data":{"path":{"text":"./a.bin"},"lines":{"text":"foo two\n"},"line_number":3,"absolute_offset":8,"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}
{"type":"end","data":{"path":{"text":"./a.bin"},"binary_offset":4,"stats":{}}}
{"type":"begin","data":{"path":{"text":"./b.bin"}}}
{"type":"match","data":{"path":{"text":"./b.bin"},"lines":{"text":"foo three\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}
{"type":"end","data":{"path":{"text":"./b.bin"},"binary_offset":4,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// mixedBinaryStream pairs a binary-excluded file with a retained one:
// three match events arrive but only one stop survives filtering.
const mixedBinaryStream = `{"type":"begin","data":{"path":{"text":"./a.bin"}}}
{"type":"match","data":{"path":{"text":"./a.bin"},"lines":{"text":"foo one\n"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}
{"type":"match","data":{"path":{"text":"./a.bin"},"lines":{"text":"foo two\n"},"line_number":3,"absolute_offset":8,"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}
{"type":"end","data":{"path":{"text":"./a.bin"},"binary_offset":4,"stats":{}}}
{"type":"begin","data":{"path":{"text":"./ok.txt"}}}
{"type":"match","data":{"path":{"text":"./ok.txt"},"lines":{"text":"foo text\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}
{"type":"end","data":{"path":{"text":"./ok.txt"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"elapsed_total":{},"stats":{}}}
`

// centered returns the exact composed frame for the no-results screen:
// the line padded to the middle column of an 80x24 terminal, wrapped
// in the dark scheme's base colours.
func centered(text string) string {
	return "\x1b[37;40m" + strings.Repeat("\n", 11) +
		strings.Repeat(" ", (80-len(text))/2) + text + "\x1b[0m"
}

// A complete rg-1 stream with no matches presents the centred
// "No results found" screen — no binary suffix when nothing was
// excluded.
func TestEmptySearchShowsNoResultsScreen(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(emptyStream), Code: 1}}, options{})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(runCollectCmd(t, m))
	if v := viewText(m); v != centered("No results found") {
		t.Fatalf("no-results view = %q, want the centred screen %q", v, centered("No results found"))
	}
}

// q on the no-results screen exits 1 through the Issue #4 cleanup path:
// the child is terminated and reaped before the quit message issues.
func TestQOnNoResultsExitsOne(t *testing.T) {
	reaped := make(chan Result, 1)
	child := newKillChild(Result{Stdout: []byte(emptyStream), Code: 1})
	m := newTestModel(child, options{reap: func(r Result) { reaped <- r }})
	m.Update(searchDoneMsg{res: Result{Code: 1}, idx: searchindex.Build([]byte(emptyStream), "/wd")})

	_, cmd := m.Update(keyQ)
	if !m.quitting {
		t.Fatal("q on the no-results screen did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 1 {
		t.Fatalf("status = %d, want the empty search's exit 1", m.status)
	}
	select {
	case r := <-reaped:
		if r.Code != 1 {
			t.Fatalf("reaped code = %d, want the child's 1", r.Code)
		}
	default:
		t.Fatal("wait/reap path did not run before quit")
	}
}

// When every matched file was excluded as binary, the screen explains
// the empty list: "No results found (N binary files skipped)" with the
// distinct-file count. That applies to an all-filtered rg-0 stream and
// to an rg-1 exit alike; q still exits 1.
func TestAllBinarySearchShowsSkipCount(t *testing.T) {
	for _, code := range []int{0, 1} {
		m := newTestModel(fakeChild{res: Result{Stdout: []byte(allBinaryStream), Code: code}}, options{})
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m.Update(runCollectCmd(t, m))
		want := centered("No results found (2 binary files skipped)")
		if v := viewText(m); v != want {
			t.Fatalf("rg code %d: no-results view = %q, want %q", code, v, want)
		}
		_, cmd := m.Update(keyQ)
		runQuittingCmd(t, cmd)
		if m.status != 1 {
			t.Fatalf("rg code %d: status = %d, want 1", code, m.status)
		}
	}
}

// A stream with one binary-excluded file and one retained file browses
// normally: usable results counts retained stops after filtering — one
// here, not the three match events received — so the excluded file is
// absent and the retained file lists.
func TestMixedBinaryStreamBrowses(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(mixedBinaryStream), Code: 0}}, options{})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(runCollectCmd(t, m))
	if m.idx == nil {
		t.Fatal("index missing after a completed search")
	}
	if got := m.idx.UsableResults(); got != 1 {
		t.Fatalf("usable results = %d, want 1 retained stop", got)
	}
	v := viewText(m)
	if strings.Contains(v, "a.bin") {
		t.Fatalf("view = %q, the binary-excluded file must not appear", v)
	}
	if !strings.Contains(v, "ok.txt") {
		t.Fatalf("view = %q, want the retained file in the list", v)
	}
	if strings.Contains(v, "No results found") {
		t.Fatalf("view = %q, want browsing, not the no-results screen", v)
	}
}

// Esc on the no-results screen is a no-op: no overlay exists to dismiss
// and Esc never quits from a base state.
func TestEscOnNoResultsNoOp(t *testing.T) {
	m := newTestModel(fakeChild{res: Result{Stdout: []byte(emptyStream), Code: 1}}, options{})
	m.Update(runCollectCmd(t, m))
	_, cmd := m.Update(keyEsc)
	if cmd != nil {
		t.Fatal("Esc on the no-results screen returned a command, want a no-op")
	}
	if m.quitting {
		t.Fatal("Esc began a controlled exit; want a no-op")
	}
	if v := viewText(m); !strings.Contains(v, "No results found") {
		t.Fatalf("view = %q, want the no-results screen still up", v)
	}
}

// ctrl+c on the no-results screen overrides the fixed exit-1 outcome
// with 130 through the same cleanup path.
func TestCtrlCOnNoResultsExits130(t *testing.T) {
	m := newTestModel(newKillChild(Result{Stdout: []byte(emptyStream), Code: 1}), options{})
	m.Update(searchDoneMsg{res: Result{Code: 1}, idx: searchindex.Build([]byte(emptyStream), "/wd")})
	_, cmd := m.Update(keyCtrlC)
	if !m.quitting {
		t.Fatal("ctrl+c on the no-results screen did not begin a controlled exit")
	}
	runQuittingCmd(t, cmd)
	if m.status != 130 {
		t.Fatalf("status = %d, want ctrl+c's 130 override", m.status)
	}
}
