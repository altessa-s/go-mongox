// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package keyset reads signed keyset pages from MongoDB collections.
//
// FindPage binds the sort, prepared BSON filter, subject and timestamp precision to each continuation. Set
// Query.Registry and Query.BSONOptions to the effective encoding settings. The collection decodes results. Supply a
// stable unique TiebreakPath (default _id) and keep all sort fields in the projection. With a nil signer a single
// page can be read.
//
// # Usage
//
//	page, err := keyset.FindPage[Widget](ctx, coll, signer, keyset.Query{Limit: 50})
package keyset
