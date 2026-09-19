package app

import (
	"os"
	"strings"
	"testing"

	"vrg/internal/docs"
	"vrg/internal/searchindex"
)

// readmeText reads the repository README — the single user-facing
// documentation artifact (Issue #34).
func readmeText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatalf("README.md: %v", err)
	}
	return string(b)
}

// Every row of the Issue #31 binding table appears in the README as a
// table row, so the documented keys cannot drift from the bindings the
// model actually routes.
func TestReadmeListsEveryBinding(t *testing.T) {
	readme := readmeText(t)
	for _, b := range helpBindings {
		row := "| " + b.keys + " | " + b.desc + " |"
		if !strings.Contains(readme, row) {
			t.Fatalf("README missing binding row %q", row)
		}
	}
}

// The help overlay footer and the README carry the same scale,
// record-limit, and memory statements from the shared docs source:
// every statement and scale item must appear verbatim in the composed
// help body and in the committed README, so neither sink can drift
// from the other and deleting any statement fails the suite.
func TestHelpFooterSharesReadmeLimits(t *testing.T) {
	readme := readmeText(t)
	m := helpBrowseModel(t)
	m.Update(keyH)
	if !m.helpOpen {
		t.Fatal("h did not open help")
	}
	body := m.help.text
	for _, stmt := range docs.Statements {
		if !strings.Contains(body, stmt) {
			t.Fatalf("help body missing statement %q:\n%s", stmt, body)
		}
		if !strings.Contains(readme, stmt) {
			t.Fatalf("README missing statement %q", stmt)
		}
	}
	for _, item := range docs.ScaleItems {
		if !strings.Contains(body, item) {
			t.Fatalf("help body missing scale item %q:\n%s", item, body)
		}
		if !strings.Contains(readme, item) {
			t.Fatalf("README missing scale item %q", item)
		}
	}
}

// The README's exit-status documentation covers all four statuses:
// both exit-0 reasons (successful search and browse, and command-line
// help), the no-results 1, the pre-TUI usage/root/start failures and
// fatal outcomes under 2 — including the flags-only missing-pattern
// usage error — and the cancellation 130s (q while searching or
// result preparation is incomplete, ctrl+c in any state).
func TestReadmeExitStatuses(t *testing.T) {
	readme := readmeText(t)
	rows := []struct {
		status string
		tokens []string
	}{
		{"0", []string{"help"}},
		{"1", []string{"No results"}},
		{"2", []string{"usage", "root", "vrg -i"}},
		{"130", []string{"q", "ctrl+c"}},
	}
	for _, r := range rows {
		var line string
		for _, l := range strings.Split(readme, "\n") {
			if strings.HasPrefix(l, "| "+r.status+" |") {
				line = l
				break
			}
		}
		if line == "" {
			t.Fatalf("README has no exit-status row for %s", r.status)
		}
		for _, tok := range r.tokens {
			if !strings.Contains(line, tok) {
				t.Fatalf("README exit-status row %q missing %q", line, tok)
			}
		}
	}

	// The documented statuses are the outcome function's outputs:
	// each search-derived condition below produces the status the
	// README documents for it (Issue #9's DecideOutcome).
	cases := []struct {
		name string
		in   OutcomeInput
		want int
	}{
		{"clean search with usable results", OutcomeInput{
			Result: Result{Code: 0}, Integrity: searchindex.Integrity{Complete: true}, Usable: 3}, 0},
		{"clean search, nothing usable", OutcomeInput{
			Result: Result{Code: 1}, Integrity: searchindex.Integrity{Complete: true}, Usable: 0}, 1},
		{"fatal process code, usable results", OutcomeInput{
			Result: Result{Code: 3}, Integrity: searchindex.Integrity{Complete: true}, Usable: 2}, 2},
		{"fatal process code, nothing usable", OutcomeInput{
			Result: Result{Code: 3}, Integrity: searchindex.Integrity{Complete: true}, Usable: 0}, 2},
		{"integrity failure with results", OutcomeInput{
			Result: Result{Code: 0}, Integrity: searchindex.Integrity{Complete: false}, Usable: 4}, 2},
		{"record loss, nothing usable", OutcomeInput{
			Result: Result{Code: 0}, Integrity: searchindex.Integrity{Complete: true}, Usable: 0,
			RecordLoss: RecordLoss{Malformed: 1}}, 2},
		{"anomalous rg 1 with retained results", OutcomeInput{
			Result: Result{Code: 1}, Integrity: searchindex.Integrity{Complete: true}, Usable: 2}, 0},
	}
	for _, c := range cases {
		if got := DecideOutcome(c.in).Status; got != c.want {
			t.Fatalf("%s: DecideOutcome().Status = %d, want the documented %d", c.name, got, c.want)
		}
	}
}
