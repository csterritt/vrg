// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
	"vrg/internal/cli"
	"vrg/internal/present"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run is the process boundary: it alone chooses exit statuses and output
// streams. The CLI module returns an explicit result kind; the library
// never exits the process and never writes to either stream.
func run(args []string, stdout, stderr io.Writer) int {
	res := cli.Parse(args, stdout, cli.Env{Stat: os.Stat})
	switch res.Kind {
	case cli.KindHelp:
		return 0
	case cli.KindSearch:
		return runSearch(res, stderr)
	default:
		fmt.Fprintln(stderr, res.Diagnostic)
		fmt.Fprintf(stderr, "\n%s", cli.HelpText())
		return 2
	}
}

// diagSnapshot is the process boundary's own copy of the session
// diagnostic collection: the model feeds it through Config.OnCollect as
// each diagnostic is collected, so it survives whatever final model —
// or none — program.Run() returns. Collection happens only inside
// Update, so the snapshot is complete once Run() has returned; reading
// it afterwards needs no synchronisation.
type diagSnapshot []string

// collect appends one sanitized diagnostic line to the snapshot.
func (s *diagSnapshot) collect(d string) { *s = append(*s, d) }

// replay writes the snapshot to w, one line per collected occurrence in
// collection order — the common writer every controlled exit funnels
// through; there is no separate direct write.
func (s diagSnapshot) replay(w io.Writer) {
	for _, d := range s {
		fmt.Fprintln(w, d)
	}
}

// finalModelDiagnostic names the invalid-final-model condition: Run()
// returned no model vrg can read — nil, or a model of an unexpected
// type. A failing return shape is never a silent exit, so this
// diagnostic joins the replayed snapshot in place of the missing
// model's collection.
func finalModelDiagnostic(fm tea.Model) string {
	if fm == nil {
		return "vrg: program returned a nil final model"
	}
	return "vrg: program returned an unexpected final model"
}

// runSearch spawns rg from the invocation working directory and runs the
// TUI. A start failure is a sanitized stderr diagnostic and exit 2
// before the TUI exists. Every Run() return shape — ordinary quit,
// cancellation, program error, absent or wrong-type final model —
// funnels through one shutdown sequence: Run() returns with the
// terminal already restored, the child is terminated and reaped before
// the status is decided, and only then does the diagnostic snapshot
// replay to stderr — session diagnostics in collection order, then the
// invalid-final-model diagnostic when applicable, then the runtime
// error exactly once. Every failing shape is a controlled application
// failure: exit 2, matching the startup-failure convention.
func runSearch(res cli.Result, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: %s\n", present.Diagnostic(err.Error()))
		return 2
	}
	var snap diagSnapshot
	cfg := app.Config{
		Rg:        "rg",
		Argv:      res.ChildArgv,
		Workdir:   wd,
		OnCollect: snap.collect,
	}
	progCtx, cleanup := wireTestHooks(&cfg)
	defer cleanup()
	sess, err := app.Start(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(stderr, "vrg: %s\n", present.Diagnostic(err.Error()))
		return 2
	}
	// runProgram is the program-runner boundary: the untagged build
	// delegates straight to tea.NewProgram().Run(); the vrg_testhooks
	// build can substitute the returned (final model, error) tuple.
	fm, runErr := runProgram(sess.Model(), progCtx)

	// Run returned, so the display and input modes are already restored.
	// Terminate the child if it is still running and wait for the
	// collector's reap before leaving — no orphan, no zombie.
	sess.Cancel()
	<-sess.Reaped()

	model, ok := fm.(app.Model)
	switch {
	case runErr == nil && ok:
		snap.replay(stderr)
		return model.ExitCode()
	case ok && errors.Is(runErr, tea.ErrInterrupted):
		// Interrupt arrives here when ctrl+c was a real SIGINT rather
		// than a raw-mode keystroke the model handled.
		snap.replay(stderr)
		return 130
	default:
		// A controlled application failure. Session diagnostics
		// replay from the snapshot even when the final-model
		// assertion failed; the model-shape and runtime-error
		// diagnostics join the snapshot so the single writer replays
		// everything — exactly once across both mechanisms.
		if !ok {
			snap.collect(finalModelDiagnostic(fm))
		}
		if runErr != nil {
			snap.collect(present.Diagnostic("vrg: " + runErr.Error()))
		}
		snap.replay(stderr)
		return 2
	}
}
