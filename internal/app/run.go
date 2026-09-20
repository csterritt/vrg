package app

import (
	"errors"
	"fmt"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/safepresentation"
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
	// Runner, when non-nil, runs the constructed Bubble Tea program in
	// place of a direct Run call. Unlike Program — a substitution that
	// replaces the whole program — Runner wraps the real program, so
	// diagnostic delivery and the Fail trigger still apply. It is the
	// executable's build-tagged runner seam; nil runs the program
	// directly.
	Runner func(*tea.Program) (tea.Model, error)
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
	// OnCollect, when non-nil, is invoked once per diagnostic line at
	// the moment it is processed into the session collection — the
	// acknowledgement side channel tests wait on before sending an exit
	// key, proving collection rather than mere child output.
	OnCollect func(string)
	// OnEvent, when non-nil, receives each acknowledgement record the
	// model's Update emits — one per processed message, one per
	// processed key press, and one per committed transition (a state
	// entered, an overlay opened or dismissed, a load applied, a
	// layout installed) — each with a per-process monotonic sequence
	// number so a test waits on the exact occurrence its action
	// caused rather than a stale same-kind record. The records observe
	// the model's transitions; they never change them.
	OnEvent func(Event)
}

// Run executes a validated search invocation end to end: it starts rg
// from the invocation working directory, runs the TUI, and returns the
// process exit status. A child that cannot start produces a sanitized
// stderr diagnostic and exit 2 without entering the TUI. Every exit —
// ordinary, cancelled, or a controlled application failure — terminates
// and reaps a still-running child after the program has restored the
// terminal, and only then replays the session's collected diagnostics
// to stderr. A returned final model that is absent or the wrong type is
// itself a controlled application failure: the replay names the
// condition and the exit is 2 rather than a silent zero.
func Run(argv []string, env Env) int {
	stderr := env.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	workdir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: cannot determine the working directory: %s\n", safepresentation.EscapePath([]byte(err.Error())))
		return 2
	}
	start := env.Start
	if start == nil {
		start = execStarter
	}
	child, err := start(argv, workdir)
	if err != nil {
		fmt.Fprintf(stderr, "vrg: cannot start rg: %s\n", safepresentation.EscapePath([]byte(err.Error())))
		return 2
	}

	proc := &reaper{child: child, onReap: env.OnReap}
	m := New(proc, workdir)
	m.gate = env.Gate
	m.diags.onAdd = env.OnCollect
	m.acks.emit = env.OnEvent
	final, err := runProgram(m, env)

	// Cleanup boundary, common to ordinary quits, cancellation, and
	// controlled failures: the program has already returned and restored
	// the terminal, so terminate and reap a still-running child before
	// deciding the exit status.
	proc.Terminate()
	_ = proc.Wait()

	// Replay boundary, one ordered sequence for every Run() return
	// shape: the session diagnostics the shared collection retained —
	// m.diags survives independently of the final-model assertion — then
	// a diagnostic naming an absent or wrong-type final model, then the
	// runtime error. Both post-run diagnostics enter the same collection
	// rather than a separate direct write, so the single
	// post-restoration writer emits each exactly once, in order. An
	// interrupted run is an ordinary cancellation: no failure
	// diagnostics apply.
	interrupted := errors.Is(err, tea.ErrInterrupted)
	fm, usable := final.(Model)
	if !interrupted {
		if !usable {
			m.diags.add("vrg: program ended without a usable final model")
		}
		if err != nil {
			m.diags.add("vrg: " + safepresentation.EscapePath([]byte(err.Error())))
		}
	}
	m.diags.replay(stderr)

	switch {
	case interrupted:
		return 130
	case err != nil || !usable:
		return 2
	}
	return fm.status
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
	// Diagnostic messages the collection path drains from the child are
	// delivered back into the program's own message queue.
	m.diags.emit = prog.Send
	run := prog.Run
	if env.Runner != nil {
		run = func() (tea.Model, error) { return env.Runner(prog) }
	}
	if env.Fail == nil {
		return run()
	}
	type outcome struct {
		model tea.Model
		err   error
	}
	out := make(chan outcome, 1)
	go func() {
		fm, err := run()
		out <- outcome{fm, err}
	}()
	select {
	case r := <-out:
		return r.model, r.err
	case <-env.Fail:
		prog.Kill()
		// Run must have fully returned — and restored the terminal —
		// before the failure is reported. Its last model is a real
		// Model, so the shutdown sequence does not mistake a killed run
		// for an absent final model.
		r := <-out
		return r.model, controlledFailure(env.FailDiagnostic)
	}
}

// controlledFailure is the error an injected failure trigger reports.
func controlledFailure(diag string) error {
	if diag == "" {
		diag = "controlled application failure"
	}
	return errors.New(diag)
}
