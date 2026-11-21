// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package bsonbound bounds the BSON an encoder writes. The driver's document writer buffers a whole document before it hands it
// to its io.Writer, so a limit on that io.Writer runs only after the document is built: a value that expands — a small graph
// sharing its parts, an encoder that writes a large document for a small value — is built in memory first. [NewValueWriter] sits
// between the encoders and the driver's writer instead: every value is measured, and charged to an optional budget, before it
// is forwarded, so the encoding fails with [ErrLimitExceeded] as soon as the running size passes the bound, with no more than
// the bound buffered.
//
// # Usage
//
//	raw, err := bsonbound.Marshal(bson.D{{Key: "v", Value: v}}, maxBytes, maxNesting, func(enc *bson.Encoder) {
//		enc.SetRegistry(reg)
//	})
package bsonbound
