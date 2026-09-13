// Command vrg runs ripgrep and browses the results in a terminal UI.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

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

	// Test seam: if VRG_TEST_REAP is set, write the reaped wait status
	// to that file path after the child exits, proving vrg's Wait/reap
	// path ran.
	if reapFile := os.Getenv("VRG_TEST_REAP"); reapFile != "" {
		proc.OnReap = func(waitErr error) {
			status := "exited"
			if waitErr != nil {
				status = waitErr.Error()
			}
			_ = os.WriteFile(reapFile, []byte(status), 0o644)
		}
	}

	opts := []app.Option{app.WithProcess(proc)}

	// Test seam: if VRG_TEST_GATE is set, hold index preparation until
	// the named file appears.
	if gateFile := os.Getenv("VRG_TEST_GATE"); gateFile != "" {
		gate := make(chan struct{})
		go func() {
			for {
				if _, err := os.Stat(gateFile); err == nil {
					close(gate)
					return
				}
			}
		}()
		opts = append(opts, app.WithGate(gate))
	}

	// Test seam: if VRG_TEST_FAIL_TRIGGER is set, watch for that file
	// to appear and trigger a controlled failure with the diagnostic
	// from VRG_TEST_FAIL_DIAGNOSTIC.
	if failTrigger := os.Getenv("VRG_TEST_FAIL_TRIGGER"); failTrigger != "" {
		failCh := make(chan string, 1)
		diag := os.Getenv("VRG_TEST_FAIL_DIAGNOSTIC")
		if diag == "" {
			diag = "vrg: controlled failure"
		}
		go func() {
			for {
				if _, err := os.Stat(failTrigger); err == nil {
					failCh <- diag
					return
				}
			}
		}()
		opts = append(opts, app.WithFailureSignal(failCh))
	}

	// Test seam: if VRG_TEST_DIAGNOSTIC_TRIGGER is set, watch for that
	// file to appear and emit a DiagnosticMsg with the text from
	// VRG_TEST_DIAGNOSTIC_TEXT (Issue #11). This lets PTY tests emit a
	// diagnostic while the fake rg or preparation gate is still
	// blocked, then wait for the collection acknowledgement before
	// sending the exit key.
	if diagTrigger := os.Getenv("VRG_TEST_DIAGNOSTIC_TRIGGER"); diagTrigger != "" {
		diagCh := make(chan string, 1)
		diagText := os.Getenv("VRG_TEST_DIAGNOSTIC_TEXT")
		if diagText == "" {
			diagText = "vrg: test diagnostic"
		}
		go func() {
			for {
				if _, err := os.Stat(diagTrigger); err == nil {
					diagCh <- diagText
					return
				}
			}
		}()
		opts = append(opts, app.WithDiagnosticSignal(diagCh))
	}

	// Test seam: if VRG_TEST_COLLECT_ACK is set, wire the onCollect
	// callback to append the sanitized diagnostic to that file (Issue
	// #11). This is the application-side acknowledgement side channel,
	// in the same mechanism family as VRG_TEST_REAP: PTY tests wait for
	// the ack file to reach the expected line count before sending the
	// exit key, proving the model has processed the diagnostic into the
	// session collection rather than merely that bytes reached the
	// pipe.
	if ackFile := os.Getenv("VRG_TEST_COLLECT_ACK"); ackFile != "" {
		_ = os.WriteFile(ackFile, nil, 0o644) // create empty file
		opts = append(opts, app.WithOnCollect(func(diag string) {
			f, err := os.OpenFile(ackFile, os.O_APPEND|os.O_WRONLY, 0o644)
			if err != nil {
				return
			}
			defer f.Close()
			fmt.Fprintln(f, diag)
		}))
	}

	model := app.New(res.ChildArgs, workdir, opts...)

	program := tea.NewProgram(model, tea.WithOutput(stdout))

	finalModel, err := program.Run()

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
