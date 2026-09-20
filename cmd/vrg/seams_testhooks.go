//go:build vrg_testhooks

package main

import (
	"os"
	"time"

	"vrg/internal/app"
)

// testSeamEnv is the testhooks half of the test-hook boundary: it reads
// the option-related VRG_TEST_* variables — a preparation gate, a reap
// side channel, a diagnostic-collection acknowledgement side channel,
// and a controlled-failure trigger — and wires them into the search
// environment. It exists only in the vrg_testhooks build; stop releases
// any file watchers it started.
func testSeamEnv(env app.Env, stop <-chan struct{}) app.Env {
	if path := os.Getenv("VRG_TEST_GATE"); path != "" {
		env.Gate = fileTrigger(path, stop)
	}
	if path := os.Getenv("VRG_TEST_REAP"); path != "" {
		env.OnReap = func(err error) {
			status := "exit status 0"
			if err != nil {
				status = err.Error()
			}
			_ = os.WriteFile(path, []byte(status+"\n"), 0o644)
		}
	}
	if path := os.Getenv("VRG_TEST_COLLECT_ACK"); path != "" {
		env.OnCollect = func(line string) {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err == nil {
				_, _ = f.WriteString(line + "\n")
				_ = f.Close()
			}
		}
	}
	if path := os.Getenv("VRG_TEST_FAIL_TRIGGER"); path != "" {
		env.Fail = fileTrigger(path, stop)
		env.FailDiagnostic = os.Getenv("VRG_TEST_FAIL_DIAGNOSTIC")
	}
	return env
}

// fileTrigger returns a channel that closes once path exists, polled on
// a paced ticker rather than a spin. The watcher exits without closing
// the channel when stop closes.
func fileTrigger(path string, stop <-chan struct{}) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		for {
			if _, err := os.Stat(path); err == nil {
				close(ch)
				return
			}
			select {
			case <-stop:
				return
			case <-t.C:
			}
		}
	}()
	return ch
}
