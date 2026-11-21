// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"testing"
	"time"
)

// coverageSlowdown multiplies every wall-clock bound when coverage instruments the binary. make test-all, the test step of CI, runs
// -race -shuffle=on -covermode=atomic -coverpkg=./...: an atomic counter on every statement, synchronized by the race detector and
// contended by the parallel tests, slows the timed loops beyond the race detector alone, by a factor the load of the machine decides.
// The factor was measured against the bounds the callers passed under -race: three
// such runs of the root and backend/mongo modules measured a worst elapsed/bound ratio of 5.4 (keyset
// TestValidateRejectsSharedGraphs, against its 5 s race-scaled bound; every other bound stayed under 0.6), so 15 leaves 2.8x
// headroom. The multiplied bound guards against a superlinear regression, whose time grows with the input far past any constant
// factor, not against a constant-factor slowdown, which only the plain and -race runs catch.
const coverageSlowdown = 15

// requireQuick asserts the wall-clock bound of a test that bounds its time, elapsed < bound. The caller passes the bound for the
// build it runs in without coverage; requireQuick multiplies it by coverageSlowdown
// only when coverage instruments the binary. The tests also check what bounds the work, the limit error, the result and the bytes
// allocated, in every build.
func requireQuick(tb testing.TB, elapsed, bound time.Duration, msgAndArgs ...any) {
	tb.Helper()

	if testing.CoverMode() != "" {
		bound *= coverageSlowdown
	}

	if elapsed >= bound {
		tb.Log(msgAndArgs...)
		tb.Fatal("Less assertion failed for elapsed")
	}
}
