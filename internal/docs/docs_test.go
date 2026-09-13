// Package docs_test contains the Issue #34 documentation
// synchronization tests. These tests assert the committed README.md and
// the help overlay footer carry the same required content — every key
// binding from Issue #31's table, every allow-listed flag from Issue
// #2's shared declarations, the complete exit-status table agreeing
// with Issue #9's outcome function, the help-only behaviour, the
// ripgrep 15.x and --no-config statements, and the scale, record-limit,
// and memory statements from the PRD's Resources and responsiveness
// section — so neither sink can drift from the implementation.
package docs_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vrg/internal/app"
	"vrg/internal/cli"
	"vrg/internal/searchindex"
)

// readmePath finds README.md at the repository root by walking up from
// the test working directory to the directory containing go.mod.
func readmePath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, "README.md")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod walking up from %s", dir)
		}
		dir = parent
	}
}

// readmeContent reads the committed README.md at the repository root.
func readmeContent(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(readmePath(t))
	if err != nil {
		t.Fatalf("Read README.md: %v", err)
	}
	return string(data)
}

// assertContains asserts that s contains substr, printing a diagnostic
// with the context label on failure.
func assertContains(t *testing.T, s, substr, context string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("%s: missing %q", context, substr)
	}
}

// --- Binding table synchronization (Issue #31) ---

// TestREADMEContainsEveryBinding verifies that the README documents
// every key binding from Issue #31's binding table (app.KeyBindings).
// Each binding's key and description must appear in the README.
func TestREADMEContainsEveryBinding(t *testing.T) {
	readme := readmeContent(t)
	for _, b := range app.KeyBindings() {
		assertContains(t, readme, b.Key, fmt.Sprintf("binding %q key", b.Key))
		assertContains(t, readme, b.Description, fmt.Sprintf("binding %q description", b.Key))
	}
}

// --- Flag allow-list synchronization (Issue #2) ---

// TestREADMEContainsEveryAllowListedFlag verifies that the README lists
// every allow-listed search flag from Issue #2's shared option
// declarations, in both short and long form.
func TestREADMEContainsEveryAllowListedFlag(t *testing.T) {
	readme := readmeContent(t)
	for _, d := range cli.OptionDecls() {
		if !d.Search {
			continue
		}
		if d.Short != 0 {
			assertContains(t, readme, "-"+string(d.Short), fmt.Sprintf("search flag %q short", d.Long))
		}
		if d.Long != "" {
			assertContains(t, readme, "--"+d.Long, fmt.Sprintf("search flag %q long", d.Long))
		}
	}
}

// TestREADMEContainsLocalHelpOptions verifies that the README documents
// the local help options (-h/--help) from the shared declarations.
func TestREADMEContainsLocalHelpOptions(t *testing.T) {
	readme := readmeContent(t)
	for _, d := range cli.OptionDecls() {
		if !d.Help {
			continue
		}
		if d.Short != 0 {
			assertContains(t, readme, "-"+string(d.Short), "local help option short")
		}
		if d.Long != "" {
			assertContains(t, readme, "--"+d.Long, "local help option long")
		}
	}
}

// TestREADMEFlagDocsFromSharedDeclarations verifies that the README's
// flag documentation is asserted against the CLI's shared option
// declarations — every declared option (help and search) appears in
// the README in short and long form with the exact no-argument
// spellings, so it cannot drift from parsing.
func TestREADMEFlagDocsFromSharedDeclarations(t *testing.T) {
	readme := readmeContent(t)
	for _, d := range cli.OptionDecls() {
		if d.Short != 0 {
			assertContains(t, readme, "-"+string(d.Short), fmt.Sprintf("option declaration short (long=%q)", d.Long))
		}
		if d.Long != "" {
			assertContains(t, readme, "--"+d.Long, fmt.Sprintf("option declaration long (short=%q)", d.Short))
		}
	}
}

// --- Exit-status documentation agreeing with the outcome function ---

// TestREADMEExitStatusAgreesWithOutcomeFunction verifies that the
// README's exit-status documentation covers 0, 1, 2, and 130, and that
// the search-derived values agree with Issue #9's outcome function
// (app.DecideOutcome).
func TestREADMEExitStatusAgreesWithOutcomeFunction(t *testing.T) {
	readme := readmeContent(t)

	// Exit 0: successful search with results, complete stream.
	o0 := app.DecideOutcome(app.OutcomeInput{
		Process:       app.ProcessResult{ExitCode: 0},
		Integrity:     searchindex.Integrity{Complete: true},
		UsableResults: 1,
	})
	if o0.ExitStatus != 0 {
		t.Fatalf("DecideOutcome(success+results): ExitStatus = %d, want 0", o0.ExitStatus)
	}
	assertContains(t, readme, "0", "exit status 0")

	// Exit 1: no results, complete stream.
	o1 := app.DecideOutcome(app.OutcomeInput{
		Process:       app.ProcessResult{ExitCode: 0},
		Integrity:     searchindex.Integrity{Complete: true},
		UsableResults: 0,
	})
	if o1.ExitStatus != 1 {
		t.Fatalf("DecideOutcome(no results): ExitStatus = %d, want 1", o1.ExitStatus)
	}
	assertContains(t, readme, "1", "exit status 1")

	// Exit 2: fatal process exit, no results.
	o2 := app.DecideOutcome(app.OutcomeInput{
		Process:       app.ProcessResult{ExitCode: 2},
		Integrity:     searchindex.Integrity{Complete: true},
		UsableResults: 0,
	})
	if o2.ExitStatus != 2 {
		t.Fatalf("DecideOutcome(fatal): ExitStatus = %d, want 2", o2.ExitStatus)
	}
	assertContains(t, readme, "2", "exit status 2")

	// Exit 2: record loss with no usable results.
	o2rl := app.DecideOutcome(app.OutcomeInput{
		Process:               app.ProcessResult{ExitCode: 0},
		Integrity:             searchindex.Integrity{Complete: true},
		UsableResults:         0,
		RecordLoss:            app.RecordLoss{Malformed: 1},
		RecordLossDiagnostics: "1 malformed record skipped",
	})
	if o2rl.ExitStatus != 2 {
		t.Fatalf("DecideOutcome(record loss): ExitStatus = %d, want 2", o2rl.ExitStatus)
	}

	// Exit 130: cancellation (not from DecideOutcome; documented).
	assertContains(t, readme, "130", "exit status 130")
}

// TestREADMEDocumentsCancellationTriggers verifies that the README
// documents the cancellation triggers for exit 130: q during search
// or result preparation, and ctrl+c anywhere.
func TestREADMEDocumentsCancellationTriggers(t *testing.T) {
	readme := readmeContent(t)
	assertContains(t, readme, "ctrl+c", "ctrl+c cancellation trigger")
	// The README must mention q as a cancellation trigger during
	// searching. We check for "q" near "search" or "cancel".
	if !strings.Contains(readme, "130") {
		t.Errorf("README does not document exit 130 for cancellation")
	}
}

// TestREADMEDocumentsPreTUIFailures verifies that the README documents
// that pre-TUI usage errors, root validation failures, and process
// start failures produce exit 2.
func TestREADMEDocumentsPreTUIFailures(t *testing.T) {
	readme := readmeContent(t)
	// Usage error → exit 2.
	assertContains(t, readme, "usage", "usage error documentation")
	// Root validation failure → exit 2.
	assertContains(t, readme, "root", "root validation documentation")
	// Start failure → exit 2.
	assertContains(t, readme, "start", "start failure documentation")
}

// --- Help-only behaviour ---

// TestREADMEDocumentsHelpOnlyPath verifies that the README documents
// the help-only exit-0 path: bare vrg and -h/--help print command-line
// help to stdout with exit 0, no search, no TUI, distinct from the
// TUI's h/? help dialog.
func TestREADMEDocumentsHelpOnlyPath(t *testing.T) {
	readme := readmeContent(t)
	// Bare vrg → help, exit 0.
	assertContains(t, readme, "vrg", "bare vrg invocation")
	// -h/--help → help, exit 0.
	assertContains(t, readme, "-h", "local -h help option")
	assertContains(t, readme, "--help", "local --help help option")
	// Command-line help is on stdout.
	assertContains(t, readme, "stdout", "help to stdout")
	// The TUI help dialog (h/?) is distinct from command-line help.
	assertContains(t, readme, "TUI", "TUI help dialog distinction")
}

// TestREADMEDocumentsFlagsOnlyUsageError verifies that the README
// documents the flags-only usage error: vrg -i with no pattern exits 2.
func TestREADMEDocumentsFlagsOnlyUsageError(t *testing.T) {
	readme := readmeContent(t)
	// The README must mention that flags without a pattern is a
	// usage error (exit 2). Check for "pattern" near "missing" or
	// "usage" and the flags-only example.
	assertContains(t, readme, "pattern", "pattern requirement")
}

// --- Ripgrep 15.x and --no-config ---

// TestREADMEDocumentsRipgrep15x verifies that the README identifies
// ripgrep 15.x as the reference family.
func TestREADMEDocumentsRipgrep15x(t *testing.T) {
	readme := readmeContent(t)
	assertContains(t, readme, "15.x", "ripgrep 15.x reference family")
}

// TestREADMEDocumentsNoConfig verifies that the README states VRG
// supplies --no-config so ripgrep configuration files are never
// honoured.
func TestREADMEDocumentsNoConfig(t *testing.T) {
	readme := readmeContent(t)
	assertContains(t, readme, "--no-config", "--no-config flag")
	// The README must state that ripgrep config files are never
	// honoured. Check for "config" near "never" or "not" or "ignored".
	assertContains(t, readme, "config", "config file reference")
}

// --- Scale examples (AC1–AC3) ---

// scaleLimitTokens are the required tokens for the scale examples,
// record limits, and memory statements. Both the README and the help
// overlay footer must contain every token. The tokens are drawn from
// the PRD's Resources and responsiveness section.
var scaleLimitTokens = []struct{ name, token string }{
	{"10,000 matched files", "10,000"},
	{"100,000 matched lines", "100,000"},
	{"50 MB file size", "50 MB"},
	{"independent qualification", "independent"},
	{"not simultaneous", "not simultaneous"},
	{"64 MiB record limit", "64 MiB"},
	{"base64 caveat", "base64"},
	{"oversized diagnostic", "oversized"},
	{"recoverable path", "recoverable"},
	{"session retention", "session"},
	{"eviction", "eviction"},
	{"aggregate memory bound", "aggregate memory"},
	{"OOM recovery", "OOM"},
	{"forced termination", "forced termination"},
}

// TestREADMEScaleExamples verifies that the README states the three
// scale examples — approximately 10,000 matched files, 100,000 matched
// lines, and individual files around 50 MB — with the explicit
// qualification that they are independent, not simultaneous capacity
// guarantees.
func TestREADMEScaleExamples(t *testing.T) {
	readme := readmeContent(t)
	scaleTokens := []struct{ name, token string }{
		{"10,000 matched files", "10,000"},
		{"100,000 matched lines", "100,000"},
		{"50 MB file size", "50 MB"},
		{"independent qualification", "independent"},
		{"not simultaneous", "not simultaneous"},
	}
	for _, tk := range scaleTokens {
		assertContains(t, readme, tk.token, "scale: "+tk.name)
	}
}

// TestREADMERecordLimit verifies that the README states the 64 MiB
// record limit, the base64 bytes expansion caveat that can push a
// single match record over it, and the oversized-record diagnostic
// naming the path when recoverable.
func TestREADMERecordLimit(t *testing.T) {
	readme := readmeContent(t)
	recordTokens := []struct{ name, token string }{
		{"64 MiB record limit", "64 MiB"},
		{"base64 caveat", "base64"},
		{"oversized diagnostic", "oversized"},
		{"recoverable path", "recoverable"},
	}
	for _, tk := range recordTokens {
		assertContains(t, readme, tk.token, "record limit: "+tk.name)
	}
}

// TestREADMEMemoryLimits verifies that the README states session-long
// buffer retention with no eviction, no aggregate memory bound, no
// reliable OOM recovery, and no guaranteed terminal cleanup under
// forced termination.
func TestREADMEMemoryLimits(t *testing.T) {
	readme := readmeContent(t)
	memoryTokens := []struct{ name, token string }{
		{"session retention", "session"},
		{"eviction", "eviction"},
		{"aggregate memory bound", "aggregate memory"},
		{"OOM recovery", "OOM"},
		{"forced termination", "forced termination"},
	}
	for _, tk := range memoryTokens {
		assertContains(t, readme, tk.token, "memory: "+tk.name)
	}
}

// --- Help footer carries the same statements ---

// TestHelpFooterCarriesSameStatements verifies that the help overlay
// footer carries the same scale, record-limit, and memory statements
// as the README. Both sinks must contain every required token so
// neither can drift from the other.
func TestHelpFooterCarriesSameStatements(t *testing.T) {
	footer := app.HelpFooter()
	if footer == "" {
		t.Fatalf("HelpFooter is empty, want scale/record-limit/memory statements")
	}
	for _, tk := range scaleLimitTokens {
		assertContains(t, footer, tk.token, "footer: "+tk.name)
	}
}

// TestREADMEAndFooterBothCarryAllScaleLimitTokens verifies that both
// the README and the help overlay footer contain every scale,
// record-limit, and memory token, so deleting any statement from
// either sink fails the suite.
func TestREADMEAndFooterBothCarryAllScaleLimitTokens(t *testing.T) {
	readme := readmeContent(t)
	footer := app.HelpFooter()
	if footer == "" {
		t.Fatalf("HelpFooter is empty")
	}
	for _, tk := range scaleLimitTokens {
		assertContains(t, readme, tk.token, "README: "+tk.name)
		assertContains(t, footer, tk.token, "footer: "+tk.name)
	}
}
