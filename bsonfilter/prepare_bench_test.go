// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter_test

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-mongox/bsonfilter"
)

func BenchmarkPrepare(b *testing.B) {
	doc := bson.M{"id": 42, "score": bson.M{"$gt": 1, "$lt": 10}}
	for b.Loop() {
		if _, err := bsonfilter.Prepare(b.Context(), doc, nil, nil); err != nil {
			b.Fatal(err)
		}
	}
}
