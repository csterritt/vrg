package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vrg/internal/searchindex"
)

// committedHelpFooter snapshots the production default of the help
// footer's substitution slot at package initialization, before any
// test can reseat the slot with a fixture.
var committedHelpFooter = helpFooter

// readREADME returns the repository's committed user-facing README —
// the single documentation artifact Issue 34 keeps synchronized with
// the implementation.
func readREADME(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	return string(b)
}

// readmeLine returns the first README line containing token.
func readmeLine(t *testing.T, readme, token string) string {
	t.Helper()
	for _, l := range strings.Split(readme, "\n") {
		if strings.Contains(l, token) {
			return l
		}
	}
	t.Fatalf("README lacks a line containing %q", token)
	return ""
}

// Every row of the shared binding table — the same data the help
// overlay renders — appears in the README, so the documented keys
// cannot drift from the implementation.
func TestReadmeListsEveryBinding(t *testing.T) {
	readme := readREADME(t)
	for _, b := range helpBindings {
		if !strings.Contains(readme, b.keys) {
			t.Errorf("README lacks binding %q", b.keys)
		}
		if !strings.Contains(readme, b.desc) {
			t.Errorf("README lacks binding description %q", b.desc)
		}
	}
}

// The README's exit-status table covers all four statuses with their
// triggers: both reasons for exit 0, the no-results exit 1, the pre-TUI
// and fatal-search exit 2, and the two cancellation paths for 130. The
// search-derived values are asserted against the Issue 9 outcome
// function so the documented statuses cannot drift from the decision.
func TestReadmeExitStatusDocumentation(t *testing.T) {
	readme := readREADME(t)

	if l := readmeLine(t, readme, "| 0 |"); !strings.Contains(l, "help") {
		t.Errorf("exit-0 row lacks the command-line help reason: %q", l)
	}
	if l := readmeLine(t, readme, "| 1 |"); !strings.Contains(l, "no usable results") {
		t.Errorf("exit-1 row lacks the no-results reason: %q", l)
	}
	if l := readmeLine(t, readme, "| 2 |"); !strings.Contains(l, "`vrg -i`") ||
		!strings.Contains(l, "usage") || !strings.Contains(l, "root") || !strings.Contains(l, "start") {
		t.Errorf("exit-2 row lacks the pre-TUI usage/root/start failures: %q", l)
	}
	if l := readmeLine(t, readme, "| 130 |"); !strings.Contains(l, "`q`") ||
		!strings.Contains(l, "searching") || !strings.Contains(l, "ctrl+c") {
		t.Errorf("exit-130 row lacks the cancellation triggers: %q", l)
	}

	// The search-derived statuses the README documents are exactly the
	// outcome function's products.
	for _, tc := range []struct {
		name string
		in   outcomeInput
		want int
	}{
		{"successful search and browse", outcomeInput{integrity: completeStream, usable: 2}, 0},
		{"complete stream with no usable results", outcomeInput{integrity: completeStream}, 1},
		{"fatal process with usable results", outcomeInput{procErr: exitError(3), integrity: completeStream, usable: 2}, 2},
		{"fatal process without usable results", outcomeInput{procErr: exitError(3), integrity: completeStream}, 2},
		{"incomplete stream", outcomeInput{usable: 2}, 2},
		{"record loss without usable results", outcomeInput{integrity: completeStream, report: searchindex.Report{Malformed: 1}}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := decideOutcome(tc.in).status; got != tc.want {
				t.Fatalf("outcome status = %d, want the documented %d", got, tc.want)
			}
		})
	}
}

// The README documents the help-only exit-0 path — bare vrg and
// -h/--help print command-line help to stdout with no search and no
// TUI — as distinct from the TUI's h/? help dialog, and the flags-only
// missing-pattern usage error (vrg -i) exiting 2.
func TestReadmeDocumentsHelpOnlyPath(t *testing.T) {
	readme := readREADME(t)
	for _, tok := range []string{
		"no arguments",
		"`-h`", "`--help`",
		"stdout",
		"exit 0",
		"command-line help",
		"`h` or `?`", // the distinct TUI help dialog
		"`vrg -i`",   // the flags-only usage error example
		"exit 2",
	} {
		if !strings.Contains(readme, tok) {
			t.Errorf("README lacks %q for the help-only path documentation", tok)
		}
	}
}

// requiredDocStatements is the verbatim contract for Issue 34's scale,
// record-limit, and memory documentation: every fragment must appear
// in both the README and the rendered help footer, so deleting a
// statement from either sink fails here.
var requiredDocStatements = []string{
	"approximately 10,000 matched files, 100,000 matched lines, or individual files around 50 MB",
	"independent, not simultaneous capacity guarantees",
	"assumes UTF-8 content with ordinary line lengths",
	"base64 bytes expand",
	"64 MiB",
	"diagnostic names the path",
	"no eviction, no aggregate memory bound, no reliable OOM recovery",
	"no guaranteed terminal cleanup under forced termination",
}

// The help overlay footer carries the same scale, record-limit, and
// memory statements as the README: the committed footer note appears
// verbatim in the README and every required statement appears in both
// sinks, so neither can drift from the other.
func TestHelpFooterMatchesReadme(t *testing.T) {
	if committedHelpFooter == "" {
		t.Fatal("the help overlay footer slot is empty: the scale-and-limits note is not installed")
	}
	readme := readREADME(t)
	if !strings.Contains(readme, committedHelpFooter) {
		t.Fatalf("README does not carry the help footer's note verbatim:\n%s", committedHelpFooter)
	}

	prev := helpFooter
	helpFooter = committedHelpFooter
	defer func() { helpFooter = prev }()
	footer := strings.Join(openHelp().lines, "\n")
	for _, stmt := range requiredDocStatements {
		if !strings.Contains(readme, stmt) {
			t.Errorf("README lacks statement %q", stmt)
		}
		if !strings.Contains(footer, stmt) {
			t.Errorf("help footer lacks statement %q", stmt)
		}
	}
}
