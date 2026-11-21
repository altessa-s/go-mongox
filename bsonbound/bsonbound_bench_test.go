// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonbound_test

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-mongox/bsonbound"
)

func BenchmarkMarshal(b *testing.B) {
	doc := bson.D{
		{Key: "tenant", Value: "acme"},
		{Key: "$or", Value: bson.A{bson.D{{Key: "name", Value: "a"}}, bson.D{{Key: "age", Value: bson.D{{Key: "$gte", Value: int32(18)}}}}}},
	}

	for b.Loop() {
		if _, err := bsonbound.Marshal(doc, 16<<20, 128, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalLargeArray(b *testing.B) {
	ids := make(bson.A, 1000)
	for i := range ids {
		ids[i] = int32(i)
	}

	doc := bson.D{{Key: "id", Value: bson.D{{Key: "$in", Value: ids}}}}

	b.ReportAllocs()

	for b.Loop() {
		if _, err := bsonbound.Marshal(doc, 16<<20, 128, nil); err != nil {
			b.Fatal(err)
		}
	}
}
