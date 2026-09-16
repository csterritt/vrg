//go:build !vrg_testhooks

package main

import "vrg/internal/app"

// testSeamOptions is the production half of the option/process-wiring
// boundary (Issue #45): it returns no options and performs no wiring.
// Every VRG_TEST_* hook lives in the vrg_testhooks variant
// (seams_testhooks.go); the unconditional call site in runSearch keeps
// both builds identical in shape while the released artifact contains
// none of the hook names or test behaviour.
func testSeamOptions(*app.Process) []app.Option {
	return nil
}
