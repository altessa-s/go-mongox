// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package indexes resolves index names and creates indexes.
//
// Names resolves explicit and compound index names in model order. Ensure checks every name before its first server
// command and creates indexes using private option builders. An empty model list succeeds. Conflicting indexes are
// left in place.
//
// # Usage
//
//	err := indexes.Ensure(ctx, coll, models)
package indexes
