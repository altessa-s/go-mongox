// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package bsonfilter validates filter values and prepares executable BSON.
//
// Prepare accepts bson.D, bson.M, map[string]any, bson.Raw and nil. Maps at every depth are sorted. Ordered
// documents, arrays and raw BSON retain their order. Use bson.D for an embedded literal whose field order matters.
// Nil means an empty document. A nil registry uses driver defaults. Validate also accepts individual admitted values.
// AdmitScalar registers a trusted struct codec for one registry. Its encoded output is checked.
//
// # Usage
//
//	raw, err := bsonfilter.Prepare(ctx, filter, reg, opts)
package bsonfilter
