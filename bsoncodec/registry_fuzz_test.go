// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsoncodec_test

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-atlas-core/types/optional"
)

// FuzzBSONRoundTrip is the same property for the encoding MongoDB documents use.
func FuzzBSONRoundTrip(f *testing.F) {
	f.Add(0, true)
	f.Add(42, true)
	f.Add(-7, false)

	f.Fuzz(func(t *testing.T, v int, present bool) {
		original := optional.Of(v, present)

		raw, err := marshal(bson.M{"value": original})
		if err != nil {
			t.Fatal(err)
		}

		var out struct {
			Value optional.Optional[int] `bson:"value"`
		}
		if err := unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}

		if want, got := original, out.Value; !reflect.DeepEqual(want, got) {
			t.Log("the value changed on its way through BSON")
			t.Fatalf("got %#v, want %#v", got, want)
		}

		if want, got := original.IsNone(), out.Value.IsZero(); !reflect.DeepEqual(want, got) {
			t.Log("IsZero must agree with IsNone, or omitempty drops a present zero value")
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
}
