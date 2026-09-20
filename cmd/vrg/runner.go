//go:build !vrg_testhooks

package main

import tea "charm.land/bubbletea/v2"

// runTeaProgram is the production half of the program-runner boundary:
// it delegates directly to the constructed program's Run.
func runTeaProgram(p *tea.Program) (tea.Model, error) {
	return p.Run()
}
