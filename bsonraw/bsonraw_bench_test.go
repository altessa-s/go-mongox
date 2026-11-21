// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonraw_test

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-mongox/bsonraw"
)

func BenchmarkWalk(b *testing.B) {
	data := marshal(b, bson.D{
		{Key: "name", Value: "a record"},
		{Key: "tags", Value: bson.A{"x", "y", "z"}},
		{Key: "nested", Value: bson.D{{Key: "at", Value: bson.DateTime(1)}, {Key: "id", Value: bson.ObjectID{1}}}},
		{Key: "deep", Value: nested(20)},
	})

	b.ReportAllocs()

	for b.Loop() {
		if err := bsonraw.Walk(bson.TypeEmbeddedDocument, data, 100, nil); err != nil {
			b.Fatal(err)
		}
	}
}
