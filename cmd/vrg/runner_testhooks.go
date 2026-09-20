//go:build vrg_testhooks

package main

import (
	"errors"
	"os"

	tea "charm.land/bubbletea/v2"
)

// runTeaProgram is the testhooks half of the program-runner boundary:
// it delegates to the real program's Run, then applies the
// VRG_TEST_RUN_* selection to the returned tuple so subprocess tests
// can drive every Issue 46 final-model/error shape at the executable's
// actual Run() return site.
//
// VRG_TEST_RUN_FINAL_MODEL: unset or "valid" keeps the real final
// model, "nil" returns nil, "invalid" returns a non-app model.
// VRG_TEST_RUN_ERROR: unset or "real" keeps the real error, "nil"
// forces nil, and any other value is injected as a fresh error whose
// text is the value itself.
func runTeaProgram(p *tea.Program) (tea.Model, error) {
	fm, err := p.Run()
	switch os.Getenv("VRG_TEST_RUN_FINAL_MODEL") {
	case "", "valid":
	case "nil":
		fm = nil
	case "invalid":
		fm = invalidFinalModel{}
	}
	switch e := os.Getenv("VRG_TEST_RUN_ERROR"); e {
	case "", "real":
	case "nil":
		err = nil
	default:
		err = errors.New(e)
	}
	return fm, err
}

// invalidFinalModel is a valid tea.Model that is not an app.Model, so
// it exercises the wrong-type final-model branch without substituting
// a fake program.
type invalidFinalModel struct{}

func (invalidFinalModel) Init() tea.Cmd { return nil }

func (invalidFinalModel) Update(tea.Msg) (tea.Model, tea.Cmd) { return nil, nil }

func (invalidFinalModel) View() tea.View { return tea.View{} }
