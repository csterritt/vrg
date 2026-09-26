package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"vrg/internal/searchindex"
	"vrg/internal/theme"
)

// readREADME returns the repository-root README.md — the single
// user-facing documentation artifact Issue #34 keeps synchronized with
// the implementation through the tests in this file and
// internal/cli's declaration-level checks.
func readREADME(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatalf("README.md: %v", err)
	}
	return string(b)
}

// Every row of the Issue #31 binding table must appear in the README —
// the test that keeps the documented key list honest: helpBindings is
// the single source the help renderer and this test both consume.
func TestREADMEDocumentsEveryKeyBinding(t *testing.T) {
	readme := readREADME(t)
	for _, b := range helpBindings {
		if !strings.Contains(readme, b.keys) {
			t.Errorf("README lacks binding keys %q", b.keys)
		}
		if !strings.Contains(readme, b.desc) {
			t.Errorf("README lacks binding description %q", b.desc)
		}
	}
}

// The README's exit-status table must cover all four statuses with
// their triggers, and its search-derived values must agree with the
// Issue #9 outcome function: decideOutcome fixes a completed search's
// status, and every code it can produce must be a documented row.
func TestREADMEExitStatusDocumentation(t *testing.T) {
	readme := readREADME(t)

	// Each representative completed search reports its fixed status
	// through the real outcome function; the README must document every
	// status it can produce — and only the four documented statuses may
	// carry a table row.
	outcomes := []struct {
		name string
		code int
	}{
		{"clean stream with usable results", decideOutcome(outcomeInput{usable: 3}).code},
		{"anomalous rg 1 with usable results", decideOutcome(outcomeInput{waitErr: exitErr(t, 1), usable: 1}).code},
		{"complete stream with no usable results", decideOutcome(outcomeInput{}).code},
		{"fatal exit code with usable results", decideOutcome(outcomeInput{waitErr: exitErr(t, 3), usable: 2}).code},
		{"fatal exit code without usable results", decideOutcome(outcomeInput{waitErr: exitErr(t, 3)}).code},
		{"integrity failure with usable results", decideOutcome(outcomeInput{
			usable: 1,
			causes: []searchindex.Cause{{Kind: searchindex.CauseMissingSummary}},
		}).code},
		{"record loss leaving no usable results", decideOutcome(outcomeInput{malformed: 1}).code},
	}
	documented := map[int]bool{}
	for _, code := range []int{0, 1, 2, 130} {
		row := "| " + strconv.Itoa(code) + " |"
		if !strings.Contains(readme, row) {
			t.Errorf("README exit-status table lacks the %q row", row)
			continue
		}
		documented[code] = true
	}
	for _, o := range outcomes {
		if !documented[o.code] {
			t.Errorf("%s: decideOutcome fixed status %d has no README exit-status row",
				o.name, o.code)
		}
	}

	// The reasons and triggers each status must state: both exit-0
	// routes (successful search and browse; command-line help), the
	// no-results 1, the pre-TUI usage/root/start failures — including
	// the flags-only missing-pattern invocation — and fatal search
	// outcomes behind 2, and the two cancellation routes behind 130.
	for _, token := range []string{
		"command-line help", "No results found",
		"usage", "vrg -i", "root", "start",
		"`q` while searching", "result preparation", "ctrl+c",
	} {
		if !strings.Contains(readme, token) {
			t.Errorf("README exit-status documentation lacks %q", token)
		}
	}
}

// The README must document the help-only exit-0 path — bare vrg and
// -h/--help printing command-line help to stdout with exit 0, no
// search, no TUI — as distinct from the TUI's h/? help dialog, plus
// the flags-only usage error vrg -i → exit 2.
func TestREADMEDocumentsHelpOnlyPath(t *testing.T) {
	readme := readREADME(t)
	for _, token := range []string{
		"vrg [flags] pattern [root]",
		"command-line help", "stdout", "exit 0",
		"without running a search", "starting the TUI",
		"`-h`/`--help`", "`h`/`?`", "distinct",
		"vrg -i", "exit 2",
	} {
		if !strings.Contains(readme, token) {
			t.Errorf("README help-only documentation lacks %q", token)
		}
	}
}

// The README must identify ripgrep 15.x as the reference family and
// state that VRG supplies --no-config so ripgrep configuration files
// are never honoured.
func TestREADMERipgrepReference(t *testing.T) {
	readme := readREADME(t)
	for _, token := range []string{
		"ripgrep 15.x", "--no-config", "configuration files are never honoured",
	} {
		if !strings.Contains(readme, token) {
			t.Errorf("README ripgrep documentation lacks %q", token)
		}
	}
}

// The README must state the three scale examples — approximately
// 10,000 matched files, 100,000 matched lines, and individual files
// around 50 MB — with the explicit qualification that they are
// independent, not simultaneous capacity guarantees.
func TestREADMEScaleExamples(t *testing.T) {
	readme := readREADME(t)
	for _, token := range []string{
		"10,000", "100,000", "50 MB",
		"independent", "not simultaneous", "capacity guarantee",
	} {
		if !strings.Contains(readme, token) {
			t.Errorf("README scale documentation lacks %q", token)
		}
	}
}

// The README must state the 64 MiB record limit, the base64 bytes
// expansion caveat that can push a single match record over it, and
// the oversized-record diagnostic naming the path when recoverable.
func TestREADMERecordLimit(t *testing.T) {
	readme := readREADME(t)
	for _, token := range []string{
		"64 MiB", "record limit", "base64", "bytes",
		"skipped and reported", "path", "recoverable",
	} {
		if !strings.Contains(readme, token) {
			t.Errorf("README record-limit documentation lacks %q", token)
		}
	}
}

// The README must state session-long buffer retention with no
// eviction, no aggregate memory bound, no reliable OOM recovery, and
// no guaranteed terminal cleanup under forced termination.
func TestREADMEMemoryLimits(t *testing.T) {
	readme := readREADME(t)
	for _, token := range []string{
		"session", "no eviction", "aggregate memory bound",
		"OOM", "forced termination", "terminal cleanup",
	} {
		if !strings.Contains(readme, token) {
			t.Errorf("README memory-limit documentation lacks %q", token)
		}
	}
}

// The help overlay footer and the README render the same Issue #34
// statements: every installed footer entry must appear verbatim in the
// README and in the composed help text, and the rendered overlay must
// carry the required scale, record-limit, and memory tokens — so
// deleting any statement from either sink fails here.
func TestHelpFooterMatchesREADME(t *testing.T) {
	readme := readREADME(t)
	if len(helpFooter) == 0 {
		t.Fatal("helpFooter is empty — the Issue #34 footer note is not installed")
	}
	composed := strings.Join(helpLines(), "\n")
	for _, line := range helpFooter {
		if strings.TrimSpace(line) == "" {
			t.Fatal("helpFooter contains an empty entry")
		}
		if !strings.Contains(composed, line) {
			t.Errorf("composed help text lacks the footer line %q", line)
		}
		if !strings.Contains(readme, line) {
			t.Errorf("README lacks the footer line %q", line)
		}
	}
	for _, token := range []string{
		"10,000", "100,000", "50 MB", "independent",
		"64 MiB", "base64", "recoverable",
		"session", "no eviction", "aggregate memory bound",
		"OOM", "forced termination", "terminal cleanup",
	} {
		if !strings.Contains(composed, token) {
			t.Errorf("help footer lacks %q", token)
		}
		if !strings.Contains(readme, token) {
			t.Errorf("README lacks the footer's %q", token)
		}
	}

	// The footer is on the rendered overlay, not just its source: at a
	// tall frame the whole note is visible inside the border.
	m := helpModel(t)
	m, _ = pressKey(t, m, "h")
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 120, Height: 60})
	m.theme = theme.Plain()
	v := m.View().Content
	for _, token := range []string{"10,000", "100,000", "64 MiB", "OOM"} {
		if !strings.Contains(v, token) {
			t.Fatalf("rendered help overlay lacks the footer token %q: %q", token, v)
		}
	}
}
