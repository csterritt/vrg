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

// The generated help is rendered from the same option declarations that
// configure the parser and drive the preflight scan, so every declared
// option appears in every spelling it supports.
func TestHelpListsEveryDeclaredOption(t *testing.T) {
	help := renderHelp()
	for _, d := range optionDecls {
		want := "  "
		if d.short != 0 {
			want += "-" + string(d.short)
			if d.long != "" {
				want += ", "
			}
		}
		if d.long != "" {
			want += "--" + d.long
		}
		if !strings.Contains(help, want+"\t") {
			t.Errorf("generated help missing option entry %q:\n%s", want, help)
		}
	}
}
