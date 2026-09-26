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

// runSearch spawns rg from the invocation working directory and runs the
// TUI. A start failure is a sanitized stderr diagnostic and exit 2
// before the TUI exists. Every controlled exit — ordinary quit,
// cancellation, program error — funnels through one cleanup boundary:
// the child is terminated and reaped before the status is decided, and
// a controlled-failure diagnostic is written exactly once, after the
// terminal has been restored.
func runSearch(res cli.Result, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}
	cfg := app.Config{Rg: "rg", Argv: res.ChildArgv, Workdir: wd}
	progCtx, cleanup := wireTestHooks(&cfg)
	defer cleanup()
	sess, err := app.Start(context.Background(), cfg)
	if err != nil {
		fmt.Fprintf(stderr, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}
	// WithInput(os.Stdin) keeps the program reading real stdin rather
	// than opening /dev/tty when stdin is already a pipe. WithWindowSize
	// is only a fallback: a real terminal reports its own size and
	// overrides it with a resize message.
	fm, err := tea.NewProgram(sess.Model(),
		tea.WithContext(progCtx),
		tea.WithInput(os.Stdin),
		tea.WithWindowSize(80, 24),
	).Run()

	// Run returned, so the display and input modes are already restored.
	// Terminate the child if it is still running and wait for the
	// collector's reap before leaving — no orphan, no zombie.
	sess.Cancel()
	<-sess.Reaped()

	switch {
	case err == nil:
		return fm.(app.Model).ExitCode()
	case errors.Is(err, tea.ErrInterrupted):
		// Interrupt arrives here when ctrl+c was a real SIGINT rather
		// than a raw-mode keystroke the model handled.
		return 130
	default:
		// A controlled application failure: the single post-restoration
		// stderr diagnostic.
		fmt.Fprintf(stderr, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}
}
