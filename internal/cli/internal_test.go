package cli

import (
	"flag"
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
