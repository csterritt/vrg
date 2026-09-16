// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

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
		return runSearch(res, stdout, stderr)
	default:
		fmt.Fprintln(stderr, res.Diagnostic)
		fmt.Fprintf(stderr, "\n%s", cli.HelpText())
		return 2
	}
}

// runSearch starts rg, runs the Bubble Tea program, and returns the exit
// code. Start failure (rg not on PATH or exec error) prints a sanitized
// diagnostic to stderr and returns 2 without entering the TUI. Every
// ordinary exit routes through the same centralized cleanup: the child
// is terminated and reaped, and the terminal is restored by Bubble Tea
// before any diagnostic is written.
func runSearch(res cli.Result, stdout, stderr io.Writer) int {
	workdir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: cannot determine working directory: %s\n", cli.Escape(err.Error()))
		return 2
	}

	rgCmd := exec.Command("rg", res.ChildArgs...)
	rgCmd.Dir = workdir
	// Put the child in its own process group so cleanup can kill the
	// entire group, ensuring shell-script children (e.g., sleep in the
	// test fake rg) are terminated and their pipes are closed.
	rgCmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	rgStdout, err := rgCmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: cannot start ripgrep: %s\n", cli.Escape(err.Error()))
		return 2
	}
	rgStderr, err := rgCmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: cannot start ripgrep: %s\n", cli.Escape(err.Error()))
		return 2
	}

	if err := rgCmd.Start(); err != nil {
		fmt.Fprintf(stderr, "vrg: cannot start ripgrep: %s\n", cli.Escape(err.Error()))
		return 2
	}

	proc := app.NewProcess(rgCmd, rgStdout, rgStderr)

	opts := []app.Option{app.WithProcess(proc)}

	// Test-hook boundary (Issue #45): the untagged production build
	// contributes no options; the vrg_testhooks build wires the
	// explicit VRG_TEST_* hook manifest's option/process seams.
	opts = append(opts, testSeamOptions(proc)...)

	model := app.New(res.ChildArgs, workdir, opts...)

	// Program-runner boundary (Issue #45): the untagged build
	// delegates directly to tea.NewProgram(...).Run(); the
	// vrg_testhooks build may substitute the final-model/error tuple
	// at this same call site.
	finalModel, err := runProgram(model, stdout)

	// Centralized cleanup: terminate and reap the child on every exit.
	// Kill the entire process group first (ensures shell-script children
	// are terminated and their pipes are closed), then wait for the
	// collection goroutine to finish.
	if rgCmd.Process != nil {
		_ = syscall.Kill(-rgCmd.Process.Pid, syscall.SIGKILL)
	}
	proc.Cleanup()

	if err != nil {
		// Terminal is already restored by Bubble Tea. Write the
		// diagnostic exactly once, after restoration.
		fmt.Fprintf(stderr, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}

	m, ok := finalModel.(app.Model)
	if !ok {
		return 2
	}

	// Post-restoration stderr replay (Issue #11): replay each collected
	// diagnostic exactly once, in collection order, to sanitized stderr
	// after the terminal has been restored by Bubble Tea. The collection
	// is independent of what was displayed; diagnostics never shown in an
	// overlay are also replayed. The controlled-failure diagnostic is
	// routed through the collection (no separate direct write), so the
	// Issue #4 writer serves every controlled exit. Replay does not wait
	// on unrelated in-flight work: only diagnostics already processed by
	// the model before the exit are collected.
	for _, d := range m.Diagnostics() {
		fmt.Fprintln(stderr, d)
	}

	return m.ExitCode()
}
