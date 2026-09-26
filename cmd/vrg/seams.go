//go:build !vrg_testhooks

package main

import (
	"context"
	"os"

	tea "charm.land/bubbletea/v2"

	"vrg/internal/app"
)

// wireTestHooks is the production half of the option/config seam
// boundary: the release binary has no test hooks, so it only supplies
// the context the program runs under.
func wireTestHooks(*app.Config) (context.Context, func()) {
	return context.Background(), func() {}
}

// runProgram constructs and runs the Bubble Tea program — direct
// production delegation; the vrg_testhooks build wraps this site.
// WithInput(os.Stdin) keeps the program reading real stdin rather than
// opening /dev/tty when stdin is already a pipe. WithWindowSize is only
// a fallback: a real terminal reports its own size and overrides it
// with a resize message.
func runProgram(m tea.Model, ctx context.Context) (tea.Model, error) {
	return tea.NewProgram(m,
		tea.WithContext(ctx),
		tea.WithInput(os.Stdin),
		tea.WithWindowSize(80, 24),
	).Run()
}
