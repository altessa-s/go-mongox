// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

//go:build !race

package bsonfilter

// raceEnabled reports a binary built with the race detector, which [requireQuick] grants raceSlowdown.
const raceEnabled = false
