// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"context"
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
// before the TUI exists.
func runSearch(res cli.Result, stderr io.Writer) int {
	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}
	m, err := app.Start(context.Background(), app.Config{
		Rg:      "rg",
		Argv:    res.ChildArgv,
		Workdir: wd,
	})
	if err != nil {
		fmt.Fprintf(stderr, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}
	// WithInput(os.Stdin) keeps the program reading real stdin rather
	// than opening /dev/tty when stdin is already a pipe. WithWindowSize
	// is only a fallback: a real terminal reports its own size and
	// overrides it with a resize message.
	fm, err := tea.NewProgram(m,
		tea.WithInput(os.Stdin),
		tea.WithWindowSize(80, 24),
	).Run()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}
	return fm.(app.Model).ExitCode()
}
