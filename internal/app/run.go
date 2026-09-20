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
	// Gate, when non-nil, holds index preparation between child exit and
	// index readiness until it closes or the run is cancelled.
	Gate <-chan struct{}
	// Fail, when non-nil, arms the controlled-failure trigger: its
	// closing aborts a running program as a controlled application
	// failure carrying FailDiagnostic. It applies only to the default
	// runner, which owns the real program it must interrupt.
	Fail <-chan struct{}
	// FailDiagnostic is the text reported for an injected controlled
	// failure; an empty value uses a default message.
	FailDiagnostic string
	// OnReap, when non-nil, is invoked once with the child's wait status
	// at the moment the child is reaped.
	OnReap func(error)
}

// Run executes a validated search invocation end to end: it starts rg
// from the invocation working directory, runs the TUI, and returns the
// process exit status. A child that cannot start produces a sanitized
// stderr diagnostic and exit 2 without entering the TUI. Every exit —
// ordinary, cancelled, or a controlled application failure — terminates
// and reaps a still-running child after the program has restored the
// terminal, and only then reports a failure on stderr.
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

	proc := &reaper{child: child, onReap: env.OnReap}
	m := New(proc, workdir)
	m.gate = env.Gate
	final, err := runProgram(m, env)

	// Cleanup boundary, common to ordinary quits, cancellation, and
	// controlled failures: the program has already returned and restored
	// the terminal, so terminate and reap a still-running child before
	// deciding the exit status.
	proc.Terminate()
	_ = proc.Wait()

	switch {
	case errors.Is(err, tea.ErrInterrupted):
		return 130
	case err != nil:
		// The single post-restoration stderr writer: a controlled
		// application failure is reported here exactly once, after the
		// terminal is back in its prior state.
		fmt.Fprintf(stderr, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}
	if fm, ok := final.(Model); ok {
		return fm.status
	}
	return 0
}

// runProgram drives the Bubble Tea program for the model. With the
// default runner and an armed Fail trigger, the trigger races the run:
// on firing, the program is killed so its shutdown restores the
// terminal, and the call reports the injected controlled failure.
func runProgram(m Model, env Env) (tea.Model, error) {
	if env.Program != nil {
		return env.Program(m)
	}
	prog := tea.NewProgram(m)
	if env.Fail == nil {
		return prog.Run()
	}
	type outcome struct {
		model tea.Model
		err   error
	}
	out := make(chan outcome, 1)
	go func() {
		fm, err := prog.Run()
		out <- outcome{fm, err}
	}()
	select {
	case r := <-out:
		return r.model, r.err
	case <-env.Fail:
		prog.Kill()
		<-out // Run must have fully returned — and restored the terminal —
		// before the failure is reported.
		return nil, controlledFailure(env.FailDiagnostic)
	}
}

// controlledFailure is the error an injected failure trigger reports.
func controlledFailure(diag string) error {
	if diag == "" {
		diag = "controlled application failure"
	}
	return errors.New(diag)
}
