// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsoncodec

//go:generate optgen generate --type=dateOptions --option-type=DateOption --option-prefix=Date --output=date_options_gen.go

// dateOptions configures a date codec registered with [RegisterDateType].
type dateOptions struct {
	// zeroAsEmptyString stores the zero date as "" instead of null, for
	// collections that already hold empty strings for "no date".
	zeroAsEmptyString bool
}
