package main

import (
	"context"
	"io"
	"os"

	"vrg/internal/app"
)

// The VRG_TEST_* environment seams let the PTY harness observe and steer
// the real binary's cleanup boundary. Every variable is inert when
// unset; they are not user-facing features.
const (
	// envReapFile names a file that receives the child's reaped wait
	// status — evidence that vrg's wait/reap path ran.
	envReapFile = "VRG_TEST_REAP_FILE"
	// envGateFIFO names a fifo whose first writer's close releases index
	// preparation, holding the searching state across the child's exit.
	envGateFIFO = "VRG_TEST_GATE_FIFO"
	// envFailFIFO names a fifo whose first writer's close injects a
	// controlled application failure into the running program.
	envFailFIFO = "VRG_TEST_FAIL_FIFO"
)

// wireTestHooks applies the VRG_TEST_* seams to cfg and returns the
// context the program runs under plus a cleanup func.
func wireTestHooks(cfg *app.Config) (context.Context, func()) {
	ctx := context.Background()
	cleanup := func() {}
	if p := os.Getenv(envReapFile); p != "" {
		if f, err := os.Create(p); err == nil {
			cfg.ReapReport = f
			cleanup = func() { _ = f.Close() }
		}
	}
	if p := os.Getenv(envGateFIFO); p != "" {
		cfg.PrepareGate = fifoReleased(p)
	}
	if p := os.Getenv(envFailFIFO); p != "" {
		c, cancel := context.WithCancel(ctx)
		go func() { <-fifoReleased(p); cancel() }()
		ctx = c
	}
	return ctx, cleanup
}

// fifoReleased returns a channel closed once a writer has opened and
// closed the fifo at path. The open blocks until a writer appears and
// the read ends at its close, so the handshake needs no polling; a fifo
// nobody ever writes simply stays held.
func fifoReleased(path string) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		_, _ = io.Copy(io.Discard, f)
	}()
	return ch
}
