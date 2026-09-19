//go:build !vrg_testhooks

package main

import tea "charm.land/bubbletea/v2"

// runProgram is the untagged half of the program-runner seam: the
// production binary delegates directly to the constructed program.
func runProgram(p *tea.Program) (tea.Model, error) { return p.Run() }
