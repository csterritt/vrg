//go:build vrg_testhooks

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"vrg/internal/app"
)

// testSeamOptions wires the explicit vrg-consumed hook manifest as app
// options; it compiles only into the vrg_testhooks variant, so the
// released binary carries none of these names or behaviours.
// VRG_TEST_GATE=<file> holds index preparation while the file exists;
// VRG_TEST_COLLECT_ACK=<file> records a line once the child's output has
// been fully collected, so tests can observe that rg exited while
// preparation is still held. VRG_TEST_LOAD_GATE=<file> holds each file
// load — read and decode/map together — while the file exists.
// VRG_TEST_DIAGNOSTIC_TRIGGER=<file> records one line each time a
// diagnostic is processed into the session collection — the
// application-side acknowledgement that a diagnostic is collected
// before an exit key; VRG_TEST_DIAGNOSTIC_TEXT overrides the recorded
// line's content, "diag" by default. VRG_TEST_REAP=<file> records the
// child's reaped wait status once per process — the side channel
// proving vrg's wait/reap path ran. VRG_TEST_FAIL_TRIGGER=<file>
// injects a controlled failure once the file exists;
// VRG_TEST_FAIL_DIAGNOSTIC overrides the injected failure's text.
// VRG_TEST_EVENT_ACK=<file> is the Issue #48 handshake seam: the app
// appends one "<seq> <event>" line per Update-processed message and
// per awaited transition — seq monotonic per process — so a PTY test
// waits on the exact acknowledgement its action caused.
func testSeamOptions() []app.Option {
	var opts []app.Option
	if p := os.Getenv("VRG_TEST_COLLECT_ACK"); p != "" {
		opts = append(opts, app.WithCollectAck(func() { appendLine(p, "collected\n") }))
	}
	if p := os.Getenv("VRG_TEST_DIAGNOSTIC_TRIGGER"); p != "" {
		text := os.Getenv("VRG_TEST_DIAGNOSTIC_TEXT")
		if text == "" {
			text = "diag"
		}
		if !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		opts = append(opts, app.WithDiagAck(func() { appendLine(p, text) }))
	}
	if p := os.Getenv("VRG_TEST_GATE"); p != "" {
		opts = append(opts, app.WithGate(func() { waitFileGone(p) }))
	}
	if p := os.Getenv("VRG_TEST_LOAD_GATE"); p != "" {
		opts = append(opts, app.WithLoadGate(func() { waitFileGone(p) }))
	}
	if p := os.Getenv("VRG_TEST_REAP"); p != "" {
		opts = append(opts, app.WithReapReport(func(res app.Result) {
			appendLine(p, fmt.Sprintf("reaped code=%d err=%v\n", res.Code, res.Err))
		}))
	}
	if p := os.Getenv("VRG_TEST_FAIL_TRIGGER"); p != "" {
		diag := os.Getenv("VRG_TEST_FAIL_DIAGNOSTIC")
		if diag == "" {
			diag = "injected test failure \x1b[7m"
		}
		opts = append(opts, app.WithFailFunc(func() error {
			waitFileExists(p)
			return errors.New(diag)
		}))
	}
	if p := os.Getenv("VRG_TEST_EVENT_ACK"); p != "" {
		// One record per emitted event, sequence-numbered under the
		// mutex: the collection goroutine's "collected" can interleave
		// with the update path's records, and the counter keeps every
		// record's order provable.
		var mu sync.Mutex
		seq := 0
		opts = append(opts, app.WithEventAck(func(ev string) {
			mu.Lock()
			defer mu.Unlock()
			seq++
			appendLine(p, fmt.Sprintf("%d %s\n", seq, ev))
		}))
	}
	return opts
}

func appendLine(path, line string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	f.WriteString(line)
	f.Close()
}

// waitFileGone polls until path no longer exists.
func waitFileGone(path string) {
	for {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// waitFileExists polls until path exists.
func waitFileExists(path string) {
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}
