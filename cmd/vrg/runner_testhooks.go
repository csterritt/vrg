//go:build vrg_testhooks

package main

import (
	"errors"
	"io"
	"os"

	tea "charm.land/bubbletea/v2"
)

// runProgram delegates to the real Bubble Tea program, then applies
// the runner-control half of the vrg-consumed hook manifest (Issue
// #45) at the actual Run() return site. VRG_TEST_RUN_FINAL_MODEL
// selects the final-model shape: empty or "valid" keeps the real
// model, "nil" returns a nil model, and "invalid" returns a tea.Model
// that is not an app.Model. VRG_TEST_RUN_ERROR selects the error:
// empty keeps the real error, "nil" forces nil, and any other value
// becomes the injected error's text. Issue #46 drives its
// return-shape matrix through these controls.
func runProgram(model tea.Model, output io.Writer) (tea.Model, error) {
	finalModel, err := tea.NewProgram(model, tea.WithOutput(output)).Run()

	switch os.Getenv("VRG_TEST_RUN_FINAL_MODEL") {
	case "nil":
		finalModel = nil
	case "invalid":
		finalModel = invalidRunModel{}
	}

	if runErr := os.Getenv("VRG_TEST_RUN_ERROR"); runErr != "" {
		if runErr == "nil" {
			err = nil
		} else {
			err = errors.New(runErr)
		}
	}

	return finalModel, err
}

// invalidRunModel is a tea.Model that is not an app.Model, supplying
// the wrong-type final-model shape for the runner seam.
type invalidRunModel struct{}

func (invalidRunModel) Init() tea.Cmd                       { return nil }
func (invalidRunModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return invalidRunModel{}, nil }
func (invalidRunModel) View() tea.View                      { return tea.View{} }
