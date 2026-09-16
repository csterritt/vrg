//go:build vrg_testhooks

package main

import (
	"fmt"
	"os"
	"time"

	"vrg/internal/app"
)

// testSeamOptions reads the option-related half of the explicit
// vrg-consumed hook manifest (Issue #45) and returns the corresponding
// app.Option values and proc.OnReap wiring. It is compiled only into
// the vrg_testhooks variant; the production build links the inert
// complementary implementation in seams.go instead.
func testSeamOptions(proc *app.Process) []app.Option {
	var opts []app.Option

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

	// Test seam: if VRG_TEST_GATE is set, hold index preparation until
	// the named file appears.
	if gateFile := os.Getenv("VRG_TEST_GATE"); gateFile != "" {
		gate := make(chan struct{})
		watchForFile(gateFile, func() { close(gate) })
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
		watchForFile(failTrigger, func() { failCh <- diag })
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
		watchForFile(diagTrigger, func() { diagCh <- diagText })
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

	return opts
}

// watchForFile polls for path to appear at a paced interval and then
// calls fire once. The sleep between checks keeps the watcher
// non-spinning; a watcher whose file never appears lives for the
// remainder of the process, which is how the PTY tests use trigger
// files they always create.
func watchForFile(path string, fire func()) {
	go func() {
		for {
			if _, err := os.Stat(path); err == nil {
				fire()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
}
