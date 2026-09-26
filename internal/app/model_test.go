package app

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/searchindex"
)

// fixtureIndex builds a prepared index from decoded fixture records.
func fixtureIndex(t *testing.T, workdir string, records ...string) *searchindex.Index {
	t.Helper()
	ix := searchindex.New(workdir)
	for _, r := range records {
		rec, err := searchindex.DecodeRecord([]byte(r))
		if err != nil {
			t.Fatalf("DecodeRecord(%s): %v", r, err)
		}
		ix.Add(rec)
	}
	ix.Prepare()
	return ix
}

// update drives one message through Update and re-anchors the concrete
// model type.
func update(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	nm, cmd := m.Update(msg)
	return nm.(Model), cmd
}

// awaitMsg runs the Init command's wait on a goroutine and bounds the
// wait: exceeding the budget means collection deadlocked, which fails
// rather than hangs the suite.
func awaitMsg(t *testing.T, m Model, budget time.Duration) tea.Msg {
	t.Helper()
	ch := make(chan tea.Msg, 1)
	go func() { ch <- m.Init()() }()
	select {
	case msg := <-ch:
		return msg
	case <-time.After(budget):
		t.Fatal("collection did not complete within budget; suspected deadlock")
		return nil
	}
}

func keyPress(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: s, Code: []rune(s)[0]}
}

// A completed model for summary-phase tests: the search finished with a
// prepared index and the model shows the interim summary.
func completedModel(t *testing.T, files int) Model {
	t.Helper()
	var recs []string
	for f := 0; f < files; f++ {
		p := "f" + string(rune('a'+f))
		recs = append(recs,
			`{"type":"begin","data":{"path":{"text":"`+p+`"}}}`,
			`{"type":"match","data":{"path":{"text":"`+p+`"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}`,
			`{"type":"end","data":{"path":{"text":"`+p+`"},"binary_offset":null}}`,
		)
	}
	recs = append(recs, `{"type":"summary","data":{}}`)
	m := newModel(nil)
	m, _ = update(t, m, searchDoneMsg{index: fixtureIndex(t, "/w", recs...)})
	return m
}

// The searching screen covers collection: while the completion channel
// is silent the model renders "Searching…" and nothing else.
func TestSearchingScreenShownWhileCollecting(t *testing.T) {
	m := newModel(make(chan searchDoneMsg))
	if got := m.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("searching view = %q, want it to contain \"Searching…\"", got)
	}
}

// Resize messages are handled while searching without touching
// collection: the model stores the new dimensions and stays in the
// searching state.
func TestResizeDuringSearching(t *testing.T) {
	m := newModel(make(chan searchDoneMsg))
	m2, _ := update(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	if m2.width != 120 || m2.height != 40 {
		t.Fatalf("dimensions = %dx%d, want 120x40", m2.width, m2.height)
	}
	if got := m2.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("view after resize = %q, still want \"Searching…\"", got)
	}
}

// An injected search-completion message transitions the model to the
// interim summary screen: "N files, M matched lines".
func TestCompletionTransitionsToSummary(t *testing.T) {
	ix := fixtureIndex(t, "/w",
		`{"type":"match","data":{"path":{"text":"a"},"lines":{"text":"x\n"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}`,
		`{"type":"match","data":{"path":{"text":"a"},"lines":{"text":"y\n"},"line_number":4,"submatches":[{"match":{"text":"y"},"start":0,"end":1}]}}`,
		`{"type":"match","data":{"path":{"text":"b"},"lines":{"text":"z\n"},"line_number":2,"submatches":[{"match":{"text":"z"},"start":0,"end":1}]}}`,
		`{"type":"summary","data":{}}`,
	)
	m := newModel(nil)
	m2, _ := update(t, m, searchDoneMsg{index: ix})
	got := m2.View().Content
	if !strings.Contains(got, "2 files, 3 matched lines") {
		t.Fatalf("summary view = %q, want \"2 files, 3 matched lines\"", got)
	}
	if strings.Contains(got, "Searching…") {
		t.Fatalf("summary view still shows searching: %q", got)
	}
}

// On the interim summary screen q exits with status 0.
func TestQOnSummaryExitsZero(t *testing.T) {
	m := completedModel(t, 1)
	m2, cmd := update(t, m, keyPress("q"))
	if cmd == nil {
		t.Fatal("q on summary returned no command, want tea.Quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q on summary command = %T, want tea.QuitMsg", cmd())
	}
	if m2.ExitCode() != 0 {
		t.Fatalf("ExitCode = %d, want 0", m2.ExitCode())
	}
}

// Index preparation is gated independently of rg exit: after the child
// has exited and both pipes are drained, the gate holds preparation and
// the app stays in the searching state until it is released.
func TestGateHoldsSearchingAfterRgExit(t *testing.T) {
	work := t.TempDir()
	out := t.TempDir()
	t.Setenv("VRG_FAKE_RG", "echo")
	t.Setenv("VRG_FAKE_DIR", out)
	drained := make(chan struct{})
	gate := make(chan struct{})
	m, err := Start(context.Background(), Config{
		Rg:          os.Args[0],
		Argv:        []string{"--json", "--no-config", "--", "foo", "."},
		Workdir:     work,
		Drained:     drained,
		PrepareGate: gate,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// The child has fully exited and both pipes are drained; the gate is
	// still holding index preparation.
	select {
	case <-drained:
	case <-time.After(10 * time.Second):
		t.Fatal("child did not exit and drain within budget")
	}
	select {
	case <-m.done:
		t.Fatal("completion delivered while the preparation gate was held")
	default:
	}
	if got := m.View().Content; !strings.Contains(got, "Searching…") {
		t.Fatalf("view after rg exit with gate held = %q, want \"Searching…\"", got)
	}

	close(gate)
	msg := awaitMsg(t, m, 10*time.Second)
	m2, _ := update(t, m, msg)
	if got := m2.View().Content; !strings.Contains(got, "matched line") {
		t.Fatalf("view after gate release = %q, want interim summary", got)
	}
}
