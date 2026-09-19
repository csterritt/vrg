//go:build vrg_testhooks

package main

import (
	"errors"
	"os"

	tea "charm.land/bubbletea/v2"
)

// runProgram delegates to the constructed program's Run, then applies
// the VRG_TEST_RUN_* overrides so tests select every Issue #46 return
// shape at the executable's real program-runner call site:
// VRG_TEST_RUN_FINAL_MODEL=nil or invalid replaces the returned model
// (any other value keeps the real final model) and a non-empty
// VRG_TEST_RUN_ERROR replaces the returned error with that message.
func runProgram(p *tea.Program) (tea.Model, error) {
	final, err := p.Run()
	switch os.Getenv("VRG_TEST_RUN_FINAL_MODEL") {
	case "nil":
		final = nil
	case "invalid":
		final = foreignModel{}
	}
	if msg := os.Getenv("VRG_TEST_RUN_ERROR"); msg != "" {
		err = errors.New(msg)
	}
	return final, err
}

// foreignModel stands in for a wrong-type final model: it satisfies
// tea.Model but is never the app's model type, so the post-Run
// assertion fails exactly as a real wrong-shape return would.
type foreignModel struct{}

func (foreignModel) Init() tea.Cmd                       { return nil }
func (foreignModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return foreignModel{}, nil }
func (foreignModel) View() tea.View                      { return tea.NewView("") }
