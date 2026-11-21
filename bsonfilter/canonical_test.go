// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"bytes"
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// mustCanonical returns the canonical form of filter.
func mustCanonical(tb testing.TB, filter bson.D) bson.D {
	tb.Helper()

	d, err := canonicalFilter(tb.Context(), filter)
	if err != nil {
		tb.Fatal(err)
	}

	return d
}

// mapFilter returns a filter holding multi-entry maps at every position a filter admits one.
func mapFilter() bson.D {
	return bson.D{
		{Key: "score", Value: bson.M{"$gte": 1, "$lt": 10, "$ne": 5, "$exists": true}},
		{Key: "$or", Value: []bson.M{{"a": 1, "b": 2, "c": 3}, {"d": map[string]any{"x": 1, "y": 2, "z": 3}}}},
		{Key: "tags", Value: bson.M{"$in": []map[string]any{{"p": 1, "q": 2, "r": 3}}, "$nin": bson.A{"x"}}},
		{Key: "$and", Value: bson.A{bson.D{{Key: "m", Value: map[string]any{"k1": 1, "k2": 2, "k3": 3}}}}},
		{Key: "nested", Value: [][]any{{bson.M{"u": 1, "v": 2, "w": 3}}}},
	}
}

// TestCanonicalFilterIsDeterministic checks a filter of multi-entry maps encodes to the same bytes, and binds the same token,
// every time.
func TestCanonicalFilterIsDeterministic(t *testing.T) {
	t.Parallel()

	var want bson.Raw

	for i := range 200 {
		filter := mapFilter()
		if err := Validate(t.Context(), filter, nil); err != nil {
			t.Fatal(err)
		}

		raw, err := marshalFilter(t.Context(), mustCanonical(t, filter), nil, nil)
		if err != nil {
			t.Fatal(err)
		}

		if i == 0 {
			want = raw
		}

		if want, got := want, raw; !reflect.DeepEqual(want, got) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

// TestCanonicalFilterKeepsMeaning checks the canonical filter holds the same documents as the original, sorts map entries,
// keeps bson.D order and leaves the original unchanged.
func TestCanonicalFilterKeepsMeaning(t *testing.T) {
	t.Parallel()

	filter := mapFilter()
	canonical := mustCanonical(t, filter)

	if want, got := asMaps(t, filter), asMaps(t, canonical); !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := (bson.D{{Key: "$exists", Value: true}, {Key: "$gte", Value: 1}, {Key: "$lt", Value: 10}, {Key: "$ne", Value: 5}}), canonical[0].Value; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := []string{"score", "$or", "tags", "$and", "nested"}, keysOf(canonical); !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if reflect.TypeFor[bson.M]() != reflect.TypeOf(filter[0].Value) {
		t.Log("the original is not modified")
		t.Fatal("IsType assertion failed for bson.M{}")
	}
}

// asMaps returns the documents of d decoded as maps at every depth: d without its key order.
func asMaps(tb testing.TB, d bson.D) bson.M {
	tb.Helper()

	raw, err := bson.Marshal(d)
	if err != nil {
		tb.Fatal(err)
	}

	dec := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(raw)))
	dec.DefaultDocumentM()

	var out bson.M
	if err := dec.Decode(&out); err != nil {
		tb.Fatal(err)
	}

	return out
}

// keysOf returns the keys of d in order.
func keysOf(d bson.D) []string {
	out := make([]string, len(d))
	for i, e := range d {
		out[i] = e.Key
	}

	return out
}

// TestCanonicalFilterSharesOrderedValues checks a filter without a multi-entry map is returned as it is.
func TestCanonicalFilterSharesOrderedValues(t *testing.T) {
	t.Parallel()

	filter := bson.D{
		{Key: "a", Value: bson.M{"$gt": 1}},
		{Key: "b", Value: bson.D{{Key: "z", Value: 1}, {Key: "a", Value: 2}}},
		{Key: "c", Value: []string{"x", "y"}},
		{Key: "d", Value: bson.A{map[string]any{"k": 1}}},
	}

	got := mustCanonical(t, filter)
	if want, got := filter, got; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if &filter[0] != &got[0] {
		t.Fatal("Same assertion failed for &filter[0]")
	}
}
