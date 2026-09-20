package app

import (
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/cli"
)

// Env supplies Run's external dependencies; the zero value is the
// production configuration.
type Env struct {
	// Start launches the rg child; nil uses execStarter (rg on PATH).
	Start Starter
	// Stderr receives diagnostics; nil uses os.Stderr.
	Stderr io.Writer
	// Program runs the Bubble Tea program; nil uses tea.NewProgram.
	Program func(Model) (tea.Model, error)
}

// Run executes a validated search invocation end to end: it starts rg
// from the invocation working directory, runs the TUI, and returns the
// process exit status. A child that cannot start produces a sanitized
// stderr diagnostic and exit 2 without entering the TUI.
func Run(argv []string, env Env) int {
	stderr := env.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	workdir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: cannot determine the working directory: %s\n", cli.Escape(err.Error()))
		return 2
	}
	start := env.Start
	if start == nil {
		start = execStarter
	}
	child, err := start(argv, workdir)
	if err != nil {
		fmt.Fprintf(stderr, "vrg: cannot start rg: %s\n", cli.Escape(err.Error()))
		return 2
	}
	run := env.Program
	if run == nil {
		run = func(m Model) (tea.Model, error) { return tea.NewProgram(m).Run() }
	}
	final, err := run(New(child, workdir))
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			return 130
		}
		return 2
	}
	if m, ok := final.(Model); ok {
		return m.status
	}
	return 0
}
