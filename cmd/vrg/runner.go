//go:build !vrg_testhooks

package main

import (
	"io"

	tea "charm.land/bubbletea/v2"
)

// runProgram is the production half of the program-runner boundary
// (Issue #45): it constructs the Bubble Tea program and returns
// Run()'s (final model, error) tuple unchanged. The vrg_testhooks
// variant (runner_testhooks.go) can override that tuple at this same
// call site to produce the return shapes Issue #46 requires.
func runProgram(model tea.Model, output io.Writer) (tea.Model, error) {
	return tea.NewProgram(model, tea.WithOutput(output)).Run()
}
