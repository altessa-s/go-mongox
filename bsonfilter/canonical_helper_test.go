// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// canonicalFilter returns filter, which [Validate] admitted, with every bson.M and map[string]any of two or more entries,
// at any depth, replaced by a bson.D of its entries in key order. The driver encodes a map in its iteration order, which varies
// from run to run, so the same filter would encode to different bytes and a page token bound to them would not resolve on the
// next page. A bson.D, raw BSON and a map of at most one entry keep their order; a list holding a replaced map becomes a bson.A,
// which encodes as the same array, and a []bson.E, which the driver encodes as a document, becomes a bson.D, the same document.
// Every value keeps its BSON type. Other values are shared, not copied.
//
// The work is bounded by what [Validate] admitted: a map reached through several references is canonicalized once, and
// the walk stops with the context's error once ctx ends.
func canonicalFilter(ctx context.Context, filter bson.D) (bson.D, error) {
	c := canonicalizer{ctx: ctx, untilCheck: cancelCheckInterval}

	d, changed := c.d(filter)
	if c.err != nil {
		return nil, c.err
	}

	if changed {
		return d, nil
	}

	return filter, nil
}
