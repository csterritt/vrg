//go:build !vrg_testhooks

package main

import "vrg/internal/app"

// testSeamEnv is the production half of the test-hook boundary: the
// released binary carries none of the VRG_TEST_* seams, so it wires no
// options and reads no environment variables.
func testSeamEnv(env app.Env, _ <-chan struct{}) app.Env {
	return env
}
