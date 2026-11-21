// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package keyset

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-mongox/bsonfilter"
)

func BenchmarkPosition(b *testing.B) {
	keys := []sortKey{{"rank", true}, {"_id", false}}

	doc, err := bson.Marshal(bson.D{{Key: "_id", Value: bson.NewObjectID()}, {Key: "rank", Value: 42}})
	if err != nil {
		b.Fatal(err)
	}

	filter, err := bsonfilter.Prepare(b.Context(), bson.D{{Key: "group", Value: "a"}}, nil, nil)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()

	for b.Loop() {
		payload, err := encodePosition(doc, keys, time.Millisecond)
		if err != nil {
			b.Fatal(err)
		}

		values, err := decodePosition(payload, len(keys))
		if err != nil {
			b.Fatal(err)
		}

		_ = withSeek(filter, keys, values)
	}
}
