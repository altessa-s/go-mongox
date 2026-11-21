// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package bsoncodec encodes optional values, exact decimals and calendar dates.
//
// NewRegistry returns a fresh driver registry. RegisterDateType installs a codec and admits its type for filters
// using that registry. The default atlas-core civil date writes an empty string for zero. Other registered dates
// write null unless WithDateZeroAsEmptyString is set. Both representations decode to zero.
//
// # Usage
//
//	reg := bsoncodec.NewRegistry()
package bsoncodec
