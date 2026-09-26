package app

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// exitErr returns a child wait error carrying the given exit code,
// produced by running a real process so the error is the
// *exec.ExitError that collect delivers for a failed rg.
func exitErr(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit "+strconv.Itoa(code)).Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != code {
		t.Fatalf("exit-error fixture = %v, want exit status %d", err, code)
	}
	return err
}

// noResultsModel returns a model past a completed search whose stream
// yielded no usable results — the no-results screen.
func noResultsModel(t *testing.T, waitErr error, recs ...string) Model {
	t.Helper()
	m := newModel(nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = update(t, m, searchDoneMsg{
		index:   fixtureIndex(t, "/w", recs...),
		waitErr: waitErr,
	})
	return m
}

// An empty rg-1 stream — a valid summary, no matches, exit status 1 —
// ends the search on the centred no-results screen: no browse, no load
// command, the message centred in the frame.
func TestEmptyStreamShowsNoResults(t *testing.T) {
	m, cmd := update(t, newModel(nil, nil), tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd = update(t, m, searchDoneMsg{
		index:   fixtureIndex(t, "/w", `{"type":"summary","data":{}}`),
		waitErr: exitErr(t, 1),
	})
	if cmd != nil {
		t.Fatalf("empty completion produced a command %T, want none — no file to load", cmd)
	}
	v := m.View().Content
	if !strings.Contains(v, "No results found") {
		t.Fatalf("view = %q, want the no-results screen", v)
	}
	if strings.Contains(v, "binary files skipped") {
		t.Fatalf("rg-1 empty view = %q, want no binary-skip suffix", v)
	}
	if strings.Contains(v, "Searching…") || strings.Contains(v, "Loading…") {
		t.Fatalf("view = %q, want neither searching nor browse", v)
	}
	// Centred: the 16-cell message sits 32 cells into the 80-cell row,
	// on the middle of 24 rows.
	if !strings.Contains(v, strings.Repeat(" ", 32)+"No results found") {
		t.Fatalf("no-results text not horizontally centred: %q", v)
	}
	if n := strings.Count(v[:strings.Index(v, "No results")], "\n"); n != 12 {
		t.Fatalf("no-results text on row %d, want vertically centred row 12", n)
	}
}

// An rg-0 stream whose every matched file was excluded as binary shows
// the same screen with the distinct excluded-file count appended.
func TestAllBinaryStreamShowsSkipCount(t *testing.T) {
	m := noResultsModel(t, nil,
		`{"type":"begin","data":{"path":{"text":"a.bin"}}}`,
		`{"type":"match","data":{"path":{"text":"a.bin"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"a.bin"},"binary_offset":4}}`,
		`{"type":"begin","data":{"path":{"text":"b.bin"}}}`,
		`{"type":"match","data":{"path":{"text":"b.bin"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"b.bin"},"binary_offset":9}}`,
		`{"type":"summary","data":{}}`,
	)
	v := m.View().Content
	if !strings.Contains(v, "No results found (2 binary files skipped)") {
		t.Fatalf("all-binary view = %q, want the skip-count suffix", v)
	}
}

// A stream with one binary-excluded file and one retained file browses:
// usable results is the retained stops, the excluded file never appears
// in the list, and q exits 0.
func TestMixedStreamBrowsesRetained(t *testing.T) {
	dir := t.TempDir()
	writeWorkFile(t, dir, "keep.txt", "hit\n")
	m := newModel(nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, cmd := update(t, m, searchDoneMsg{index: fixtureIndex(t, dir,
		`{"type":"begin","data":{"path":{"text":"gone.bin"}}}`,
		`{"type":"match","data":{"path":{"text":"gone.bin"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"gone.bin"},"binary_offset":4}}`,
		`{"type":"begin","data":{"path":{"text":"keep.txt"}}}`,
		`{"type":"match","data":{"path":{"text":"keep.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
		`{"type":"end","data":{"path":{"text":"keep.txt"},"binary_offset":null}}`,
		`{"type":"summary","data":{}}`,
	)})
	if cmd == nil {
		t.Fatal("mixed completion returned no command, want the retained file's load")
	}
	if m.phase != phaseBrowse {
		t.Fatalf("phase = %d, want browse with one usable result", m.phase)
	}
	if len(m.stops) != 1 {
		t.Fatalf("usable results = %d stops, want 1", len(m.stops))
	}
	v := m.View().Content
	if !strings.Contains(v, "keep.txt") {
		t.Fatalf("browse view lacks the retained file: %q", v)
	}
	if strings.Contains(v, "gone.bin") {
		t.Fatalf("browse view lists the excluded file: %q", v)
	}
}

// q on the no-results screen quits through the ordinary cleanup path
// with the search's fixed status 1 — it is not a cancellation.
func TestQOnNoResultsExitsOne(t *testing.T) {
	cancelled := false
	m := newModel(nil, func() { cancelled = true })
	m, _ = update(t, m, searchDoneMsg{
		index:   fixtureIndex(t, "/w", `{"type":"summary","data":{}}`),
		waitErr: exitErr(t, 1),
	})
	m2, cmd := update(t, m, keyPress("q"))
	if cmd == nil {
		t.Fatal("q on no-results returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q on no-results command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 1 {
		t.Fatalf("ExitCode = %d, want 1", m2.ExitCode())
	}
	if cancelled {
		t.Fatal("q on no-results signalled child termination; want the ordinary quit path")
	}
}

// Esc on the no-results screen is a no-op: no command, no state change,
// the fixed status stays 1 and the screen stays up.
func TestEscOnNoResultsIsNoOp(t *testing.T) {
	m := noResultsModel(t, nil, `{"type":"summary","data":{}}`)
	m2, cmd := update(t, m, escPress())
	if cmd != nil {
		t.Fatalf("Esc on no-results produced a command %T, want none", cmd)
	}
	if m2.quit || m2.ExitCode() != 1 {
		t.Fatalf("Esc on no-results changed state: quit=%v code=%d", m2.quit, m2.ExitCode())
	}
	if got := m2.View().Content; !strings.Contains(got, "No results found") {
		t.Fatalf("view after Esc = %q, still want the no-results screen", got)
	}
}

// ctrl+c on the no-results screen overrides the fixed status with 130,
// the cancellation path's code.
func TestCtrlCOnNoResultsExits130(t *testing.T) {
	m := noResultsModel(t, nil, `{"type":"summary","data":{}}`)
	m2, cmd := update(t, m, ctrlCPress())
	if cmd == nil {
		t.Fatal("ctrl+c on no-results returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c on no-results command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 130 {
		t.Fatalf("ExitCode = %d, want 130", m2.ExitCode())
	}
}
