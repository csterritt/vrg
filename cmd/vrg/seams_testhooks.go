//go:build vrg_testhooks

package main

import (
	"fmt"
	"os"
	"sync"
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

	// Test seam: if VRG_TEST_UPDATE_ACK is set, append one
	// acknowledgement record per Update-processed message to that
	// file (Issue #48). Each record carries a per-process monotonic
	// sequence number, the message kind, the key label for key
	// presses, and the post-update state/overlay/dismissal
	// post-state, so PTY helpers wait on the exact transition their
	// preceding action caused rather than on elapsed time.
	if ackPath := os.Getenv("VRG_TEST_UPDATE_ACK"); ackPath != "" {
		opts = append(opts, app.WithUpdateAck(newUpdateAckLog(ackPath)))
	}

	return opts
}

// updateAckLog serializes acknowledgement records to the
// VRG_TEST_UPDATE_ACK file. The per-process monotonic seq is assigned
// here at the sink so test-side waits correlate per occurrence: an
// earlier same-kind record can never satisfy a later wait.
type updateAckLog struct {
	mu  sync.Mutex
	f   *os.File
	seq int
}

// newUpdateAckLog opens the acknowledgement sink and returns the
// callback wired into app.WithUpdateAck. An unwritable path yields an
// inert callback — the test's bounded wait then reports the missing
// acknowledgement rather than the seam fabricating records.
func newUpdateAckLog(path string) func(app.UpdateAck) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return func(app.UpdateAck) {}
	}
	l := &updateAckLog{f: f}
	return l.record
}

// record appends one acknowledgement line: seq, message kind, key
// label, and the post-update state/overlay/dismissal post-state.
func (l *updateAckLog) record(a app.UpdateAck) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.seq++
	fmt.Fprintf(l.f, "%d msg=%s key=%s state=%s overlay=%s dismissed=%t\n",
		l.seq, a.Msg, a.Key, ackStateName(a.State), ackOverlayName(a.Overlay), a.OverlayDismissed)
}

// ackStateName renders the post-update app state for an
// acknowledgement record.
func ackStateName(s app.State) string {
	switch s {
	case app.StateSearching:
		return "searching"
	case app.StateSummary:
		return "summary"
	case app.StateBrowse:
		return "browse"
	case app.StateNoResults:
		return "no-results"
	case app.StateStartFailed:
		return "start-failed"
	case app.StateFailed:
		return "failed"
	case app.StateCancelled:
		return "cancelled"
	}
	return "unknown"
}

// ackOverlayName renders the post-update overlay kind for an
// acknowledgement record.
func ackOverlayName(k app.OverlayKind) string {
	switch k {
	case app.OverlayError:
		return "error"
	case app.OverlayWarning:
		return "warning"
	case app.OverlayHelp:
		return "help"
	}
	return "none"
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
