// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonbound_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-mongox/bsonbound"
)

// units is a budget of n units.
type units struct{ n int }

func (u *units) Charge(n int) bool {
	if n > u.n {
		u.n = 0
		return false
	}

	u.n -= n

	return true
}

func TestMarshal(t *testing.T) {
	t.Parallel()

	doc := bson.D{{Key: "a", Value: bson.A{int32(1), "two", bson.D{{Key: "b", Value: true}}}}}

	want, err := bson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := bsonbound.Marshal(doc, len(want), 3, nil)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := bson.Raw(want), raw; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	_, err = bsonbound.Marshal(doc, len(want)-1, 3, nil)
	if !errors.Is(err, bsonbound.ErrLimitExceeded) {
		t.Log("one byte short")
		t.Fatalf("got %v, want %v", err, bsonbound.ErrLimitExceeded)
	}

	_, err = bsonbound.Marshal(doc, len(want), 2, nil)
	if !errors.Is(err, bsonbound.ErrLimitExceeded) {
		t.Log("one level short")
		t.Fatalf("got %v, want %v", err, bsonbound.ErrLimitExceeded)
	}

	configured := false

	_, err = bsonbound.Marshal(doc, len(want), 3, func(enc *bson.Encoder) {
		configured = true

		enc.SetRegistry(bson.NewRegistry())
	})
	if err != nil {
		t.Fatal(err)
	}

	if !configured {
		t.Fatal("expected true: configured")
	}
}

// TestMarshalStopsSharedGraphs checks a small value sharing its parts fails at the limit instead of expanding.
func TestMarshalStopsSharedGraphs(t *testing.T) {
	t.Parallel()

	var v any = bson.A{int32(1)}
	for range 40 {
		v = bson.A{v, v}
	}

	_, err := bsonbound.Marshal(bson.D{{Key: "v", Value: v}}, 1<<16, 64, nil)
	if !errors.Is(err, bsonbound.ErrLimitExceeded) {
		t.Fatalf("got %v, want %v", err, bsonbound.ErrLimitExceeded)
	}
}

// TestNewValueWriterChargesBudget checks the encoding is charged to its budget as it grows and fails once the budget is spent.
func TestNewValueWriterChargesBudget(t *testing.T) {
	t.Parallel()

	doc := bson.D{{Key: "s", Value: string(make([]byte, 1024))}}

	var buf bytes.Buffer

	budget := &units{n: 1 << 20}

	vw, bound := bsonbound.NewValueWriter(bson.NewDocumentWriter(&buf), 1<<20, 4, budget)
	if err := bson.NewEncoder(vw).Encode(doc); err != nil {
		t.Fatal(err)
	}

	if want, got := buf.Len(), bound.Size(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if bound.Depth() != 0 {
		t.Fatal("expected zero: bound.Depth()")
	}

	if budget.n >= 1<<20 {
		t.Log("the encoding was charged")
		t.Fatal("Less assertion failed for budget.n")
	}

	buf.Reset()

	vw, _ = bsonbound.NewValueWriter(bson.NewDocumentWriter(&buf), 1<<20, 4, &units{n: 1})
	if got := bson.NewEncoder(vw).Encode(doc); !errors.Is(got, bsonbound.ErrLimitExceeded) {
		t.Fatalf("got %v, want %v", got, bsonbound.ErrLimitExceeded)
	}
}
