//go:build !vrg_testhooks

package main

import "vrg/internal/app"

// testSeamOptions is the untagged half of the test-hook seam: the
// production binary wires none of the VRG_TEST_* hooks, so it always
// returns no options.
func testSeamOptions() []app.Option { return nil }
