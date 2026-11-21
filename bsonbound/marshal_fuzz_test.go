// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonbound_test

import (
	"bytes"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-mongox/bsonbound"
)

func FuzzMarshalLimit(f *testing.F) {
	f.Add([]byte("value"), uint8(0), false)
	f.Add([]byte{}, uint8(2), true)
	f.Fuzz(func(t *testing.T, data []byte, delta uint8, old bool) {
		if len(data) > 1<<16 {
			t.Skip()
		}

		subtype := byte(0)
		if old {
			subtype = 2
		}

		doc := bson.D{{Key: "x", Value: bson.A{bson.Binary{Subtype: subtype, Data: data}, string(data)}}}

		want, err := bson.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}

		limit := len(want) + int(delta%5) - 2

		got, err := bsonbound.Marshal(doc, limit, 2, nil)
		if limit < len(want) {
			if !errors.Is(err, bsonbound.ErrLimitExceeded) {
				t.Fatalf("size %d limit %d: %v", len(want), limit, err)
			}

			return
		}

		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("size %d limit %d: %v", len(want), limit, err)
		}
	})
}
