package cli

import (
	"flag"
	"strings"
	"testing"
)

// The library application must run with flag.ContinueOnError so it never
// calls os.Exit: every exit status belongs to cmd/vrg.
func TestAppUsesContinueOnError(t *testing.T) {
	app := newApp()
	if app.ErrorHandling != flag.ContinueOnError {
		t.Fatalf("app.ErrorHandling = %v, want flag.ContinueOnError", app.ErrorHandling)
	}
}

// Generated help renders every shared option declaration — the same table
// that configures mow.cli and validates the raw-token scan — so every
// allow-listed search flag appears in short and long form. A drift
// between the declarations and the rendered text fails here.
func TestGeneratedHelpListsDeclaredOptions(t *testing.T) {
	help := HelpText()
	for _, d := range optionDecls {
		want := "  " + d.spellings() + "\t" + d.desc
		if !strings.Contains(help, want) {
			t.Errorf("generated help missing option line %q:\n%s", want, help)
		}
	}
}
