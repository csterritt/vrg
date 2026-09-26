//go:build vrg_testhooks

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
)

// The VRG_TEST_* environment seams let the PTY/subprocess harness
// observe and steer the real binary's cleanup boundary. They exist only
// in this build variant — the untagged production binary compiles none
// of them — and each is inert when unset; they are not user-facing
// features.
const (
	// envReapFile names a file that receives the child's reaped wait
	// status — evidence that vrg's wait/reap path ran.
	envReapFile = "VRG_TEST_REAP"
	// envGateFIFO names a fifo whose first writer's close releases
	// index preparation, holding the searching state across the
	// child's exit.
	envGateFIFO = "VRG_TEST_GATE"
	// envFailTrigger names a fifo whose first writer's close injects a
	// controlled application failure into the running program.
	envFailTrigger = "VRG_TEST_FAIL_TRIGGER"
	// envFailDiag, when set, is the diagnostic text the injected
	// failure reports instead of the context-kill error.
	envFailDiag = "VRG_TEST_FAIL_DIAGNOSTIC"
	// envDiagTrigger names a fifo whose first writer's close injects
	// one line into the session diagnostic collection.
	envDiagTrigger = "VRG_TEST_DIAGNOSTIC_TRIGGER"
	// envDiagText is the line the diagnostic trigger injects.
	envDiagText = "VRG_TEST_DIAGNOSTIC_TEXT"
	// envCollectAck names a file that receives one acknowledgement
	// line per diagnostic processed into the session collection — the
	// application-side evidence replay tests wait on before sending an
	// exit key.
	envCollectAck = "VRG_TEST_COLLECT_ACK"
	// envEventAck names the file receiving the Issue #48
	// acknowledgement records — one "<seq> <kind> [<detail>]" line per
	// awaited model transition and key-processing boundary, with a
	// per-session monotonic sequence. The PTY harness waits on the
	// n-th matching record, so an earlier same-kind event can never
	// satisfy a later wait. The seam only observes the real
	// transitions; it never changes timing or behaviour.
	envEventAck = "VRG_TEST_EVENT_ACK"
	// envRunModel selects the final-model half of the program.Run()
	// result: "nil" returns a nil model and "invalid" a model that is
	// not app.Model; anything else keeps the real model.
	envRunModel = "VRG_TEST_RUN_FINAL_MODEL"
	// envRunError, when non-empty, replaces the program.Run() error
	// with a fresh error carrying the value's text.
	envRunError = "VRG_TEST_RUN_ERROR"
)

// failFired records that the controlled-failure trigger released, so
// the runner seam can give that failure the diagnostic
// VRG_TEST_FAIL_DIAGNOSTIC asks for.
var failFired atomic.Bool

// wireTestHooks applies the VRG_TEST_* seams to cfg and returns the
// context the program runs under plus a cleanup func.
func wireTestHooks(cfg *app.Config) (context.Context, func()) {
	ctx := context.Background()
	var closers []io.Closer
	if p := os.Getenv(envReapFile); p != "" {
		if f, err := os.Create(p); err == nil {
			cfg.ReapReport = f
			closers = append(closers, f)
		}
	}
	if p := os.Getenv(envCollectAck); p != "" {
		if f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			cfg.DiagAck = f
			closers = append(closers, f)
		}
	}
	if p := os.Getenv(envEventAck); p != "" {
		if f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			cfg.EventAck = f
			closers = append(closers, f)
		}
	}
	cleanup := func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}
	if p := os.Getenv(envGateFIFO); p != "" {
		cfg.PrepareGate = fifoReleased(p)
	}
	if p := os.Getenv(envDiagTrigger); p != "" {
		cfg.DiagInject = diagInjected(ctx, p, os.Getenv(envDiagText))
	}
	if p := os.Getenv(envFailTrigger); p != "" {
		c, cancel := context.WithCancel(ctx)
		go func() {
			<-fifoReleased(p)
			failFired.Store(true)
			cancel()
		}()
		ctx = c
	}
	return ctx, cleanup
}

// runProgram runs the real Bubble Tea program, then applies the
// VRG_TEST_RUN_* return-shape overrides — and the controlled-failure
// diagnostic — before the caller sees the result, so an injected
// (final model, error) tuple reaches the executable's actual post-Run()
// branches unchanged.
func runProgram(m tea.Model, ctx context.Context) (tea.Model, error) {
	fm, err := tea.NewProgram(m,
		tea.WithContext(ctx),
		tea.WithInput(os.Stdin),
		tea.WithWindowSize(80, 24),
	).Run()
	if failFired.Load() {
		if d := os.Getenv(envFailDiag); d != "" {
			err = errors.New(d)
		}
	}
	switch os.Getenv(envRunModel) {
	case "nil":
		fm = nil
	case "invalid":
		fm = foreignModel{}
	}
	if e := os.Getenv(envRunError); e != "" {
		err = errors.New(e)
	}
	return fm, err
}

// foreignModel is a tea.Model that is not app.Model: the "invalid final
// model" return shape VRG_TEST_RUN_FINAL_MODEL=invalid selects.
type foreignModel struct{}

func (foreignModel) Init() tea.Cmd                         { return nil }
func (f foreignModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return f, nil }
func (foreignModel) View() tea.View                        { return tea.NewView("") }

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

// diagInjected returns the channel behind Config.DiagInject: once the
// fifo at path sees its first writer open and close it delivers text as
// one diagnostic line, then closes. The send abandons under
// cancellation so an injection late in a cancelled session cannot leak
// the goroutine.
func diagInjected(ctx context.Context, path, text string) <-chan string {
	ch := make(chan string, 1)
	go func() {
		defer close(ch)
		<-fifoReleased(path)
		select {
		case ch <- text:
		case <-ctx.Done():
		}
	}()
	return ch
}
