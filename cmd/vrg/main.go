// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

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
		return runSearch(res, stdout, stderr)
	default:
		fmt.Fprintln(stderr, res.Diagnostic)
		fmt.Fprintf(stderr, "\n%s", cli.HelpText())
		return 2
	}
}

// runSearch starts rg, runs the Bubble Tea program, and returns the exit
// code. Start failure (rg not on PATH or exec error) prints a sanitized
// diagnostic to stderr and returns 2 without entering the TUI.
func runSearch(res cli.Result, stdout, stderr io.Writer) int {
	workdir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: cannot determine working directory: %s\n", cli.Escape(err.Error()))
		return 2
	}

	rgCmd := exec.Command("rg", res.ChildArgs...)
	rgCmd.Dir = workdir

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

	model := app.New(res.ChildArgs, workdir, app.WithProcess(app.Process{
		Cmd:    rgCmd,
		Stdout: rgStdout,
		Stderr: rgStderr,
	}))

	program := tea.NewProgram(model, tea.WithOutput(stdout))

	finalModel, err := program.Run()
	if err != nil {
		fmt.Fprintf(stderr, "vrg: %s\n", cli.Escape(err.Error()))
		return 2
	}

	m, ok := finalModel.(app.Model)
	if !ok {
		return 2
	}

	if m.State() == app.StateStartFailed && m.Diagnostic() != "" {
		fmt.Fprintln(stderr, m.Diagnostic())
	}

	return m.ExitCode()
}
