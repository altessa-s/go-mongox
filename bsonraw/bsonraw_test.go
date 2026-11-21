// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonraw_test

import (
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-mongox/bsonraw"
)

// marshal returns the encoding of v, a document, or fails the test.
func marshal(tb testing.TB, v any) []byte {
	tb.Helper()

	data, err := bson.Marshal(v)
	if err != nil {
		tb.Fatal(err)
	}

	return data
}

// valueOf returns the type and bytes of the value of the only element of the document holding v.
func valueOf(tb testing.TB, v any) (bson.Type, []byte) {
	tb.Helper()

	raw := bson.Raw(marshal(tb, bson.D{{Key: "v", Value: v}})).Lookup("v")

	return raw.Type, raw.Value
}

// nested returns a document holding depth levels of documents in all.
func nested(depth int) bson.D {
	doc := bson.D{}
	for range depth - 1 {
		doc = bson.D{{Key: "x", Value: doc}}
	}

	return doc
}

func TestWalkAcceptsWellFormedValues(t *testing.T) {
	t.Parallel()

	for name, v := range map[string]any{
		"null":            nil,
		"bool":            true,
		"int32":           int32(1),
		"int64":           int64(1),
		"double":          1.5,
		"string":          "text",
		"empty string":    "",
		"binary":          bson.Binary{Subtype: 0x80, Data: []byte{1, 2}},
		"old binary":      bson.Binary{Subtype: 0x02, Data: []byte{1, 2}},
		"object id":       bson.ObjectID{1},
		"date":            bson.DateTime(1),
		"timestamp":       bson.Timestamp{T: 1, I: 2},
		"decimal":         bson.NewDecimal128(1, 2),
		"regex":           bson.Regex{Pattern: "a.c", Options: "i"},
		"javascript":      bson.JavaScript("f()"),
		"symbol":          bson.Symbol("s"),
		"db pointer":      bson.DBPointer{DB: "db", Pointer: bson.ObjectID{2}},
		"code with scope": bson.CodeWithScope{Code: "f()", Scope: bson.D{{Key: "a", Value: bson.A{1, "x"}}}},
		"min key":         bson.MinKey{},
		"max key":         bson.MaxKey{},
		"undefined":       bson.Undefined{},
		"document":        bson.D{{Key: "a", Value: bson.D{{Key: "b", Value: bson.A{bson.D{}, "x"}}}}},
		"array":           bson.A{1, bson.A{}, bson.D{{Key: "k", Value: nil}}},
		"empty document":  bson.D{},
	} {
		typ, data := valueOf(t, v)
		if err := bsonraw.Walk(typ, data, 100, nil); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestWalkRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	doc := bson.TypeEmbeddedDocument

	for _, tc := range []struct {
		name string
		typ  bson.Type
		data []byte
	}{
		{"zero length", doc, []byte{0, 0, 0, 0}},
		{"length below the header", doc, []byte{4, 0, 0, 0, 0}},
		{"length beyond the bytes", doc, []byte{9, 0, 0, 0, 0}},
		{"negative length", doc, []byte{0xff, 0xff, 0xff, 0xff, 0}},
		{"overflowing length", doc, []byte{0xff, 0xff, 0xff, 0x7f, 0}},
		{"short", doc, []byte{1, 2}},
		{"empty", doc, nil},
		{"trailing bytes", doc, []byte{5, 0, 0, 0, 0, 0}},
		{"no terminator", doc, []byte{5, 0, 0, 0, 1}},
		{"unterminated key", doc, []byte{7, 0, 0, 0, 0x10, 'a', 0}},
		{"truncated element", doc, []byte{9, 0, 0, 0, 0x10, 'a', 0, 1, 0}},
		{"unknown type", doc, []byte{8, 0, 0, 0, 0x7e, 'a', 0, 0}},
		{"nested zero length", doc, []byte{12, 0, 0, 0, 0x03, 'a', 0, 0, 0, 0, 0, 0}},
		{"nested length below the header", doc, []byte{12, 0, 0, 0, 0x03, 'a', 0, 1, 0, 0, 0, 0}},
		{"nested length beyond its parent", doc, []byte{13, 0, 0, 0, 0x04, 'a', 0, 6, 0, 0, 0, 0, 0}},
		{"nested without terminator", doc, []byte{13, 0, 0, 0, 0x03, 'a', 0, 5, 0, 0, 0, 1, 0}},
		{"negative string length", doc, []byte{15, 0, 0, 0, 0x02, 'a', 0, 0xff, 0xff, 0xff, 0xff, 'x', 0, 0, 0}},
		{"zero string length", doc, []byte{12, 0, 0, 0, 0x02, 'a', 0, 0, 0, 0, 0, 0}},
		{"unterminated string", doc, []byte{14, 0, 0, 0, 0x02, 'a', 0, 2, 0, 0, 0, 'x', 'y', 0}},
		{"huge binary length", doc, []byte{16, 0, 0, 0, 0x05, 'a', 0, 0xff, 0xff, 0xff, 0x7f, 0, 'x', 0, 0, 0}},
		{"old binary without its length", doc, []byte{15, 0, 0, 0, 0x05, 'a', 0, 2, 0, 0, 0, 0x02, 1, 0, 0}},
		{"old binary with another length", doc, []byte{19, 0, 0, 0, 0x05, 'a', 0, 6, 0, 0, 0, 0x02, 9, 0, 0, 0, 1, 2, 0}},
		{"unterminated regex", doc, []byte{9, 0, 0, 0, 0x0b, 'a', 0, 'x', 0}},
		{"truncated db pointer", doc, []byte{16, 0, 0, 0, 0x0c, 'a', 0, 2, 0, 0, 0, 'x', 0, 1, 2, 0}},
		{"code with scope too short", doc, []byte{12, 0, 0, 0, 0x0f, 'a', 0, 4, 0, 0, 0, 0}},
		{"boolean above one", doc, []byte{9, 0, 0, 0, 0x08, 'a', 0, 2, 0}},
		{"truncated boolean", doc, []byte{8, 0, 0, 0, 0x08, 'a', 0, 0}},
		{"top-level boolean above one", bson.TypeBoolean, []byte{0xff}},
		{"top-level empty boolean", bson.TypeBoolean, nil},
		{"top-level zero length array", bson.TypeArray, []byte{0, 0, 0, 0}},
		{"top-level short scalar", bson.TypeInt64, []byte{1}},
		{"top-level long scalar", bson.TypeInt32, []byte{1, 0, 0, 0, 0}},
		{"top-level unknown type", bson.Type(0), nil},
		{"top-level code with scope", bson.TypeCodeWithScope, []byte{14, 0, 0, 0, 1, 0, 0, 0, 0, 4, 0, 0, 0, 0}},
	} {
		if err := bsonraw.Walk(tc.typ, tc.data, 100, nil); !errors.Is(err, bsonraw.ErrMalformed) {
			t.Errorf("%s: got %v, want ErrMalformed", tc.name, err)
		}
	}
}

func TestWalkBoundsNesting(t *testing.T) {
	t.Parallel()

	typ, data := valueOf(t, nested(10))

	if err := bsonraw.Walk(typ, data, 10, nil); err != nil {
		t.Fatalf("within the bound: %v", err)
	}

	if err := bsonraw.Walk(typ, data, 9, nil); !errors.Is(err, bsonraw.ErrTooDeep) {
		t.Fatalf("beyond the bound: got %v, want ErrTooDeep", err)
	}

	scope := bson.CodeWithScope{Code: "f()", Scope: nested(2)}

	typ, data = valueOf(t, scope)
	if err := bsonraw.Walk(typ, data, 2, nil); err != nil {
		t.Fatalf("a scope within the bound: %v", err)
	}

	if err := bsonraw.Walk(typ, data, 1, nil); !errors.Is(err, bsonraw.ErrTooDeep) {
		t.Fatalf("a scope beyond the bound: got %v, want ErrTooDeep", err)
	}
}

func TestWalkVisitsEveryElement(t *testing.T) {
	t.Parallel()

	data := marshal(t, bson.D{{Key: "a", Value: bson.A{int32(1), bson.D{{Key: "b", Value: "x"}}}}, {Key: "c", Value: true}})

	var got []string

	err := bsonraw.Walk(bson.TypeEmbeddedDocument, data, 10, func(depth int, typ bson.Type, key []byte) error {
		got = append(got, string(rune('0'+depth))+":"+typ.String()+":"+string(key))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"1:array:a", "2:32-bit integer:0", "2:embedded document:1", "3:string:b", "1:boolean:c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	stop := errors.New("stop")

	err = bsonraw.Walk(bson.TypeEmbeddedDocument, data, 10, func(int, bson.Type, []byte) error { return stop })
	if !errors.Is(err, stop) {
		t.Fatalf("got %v, want the visit's error", err)
	}
}

// FuzzWalk checks that raw BSON Walk accepts is copied by the driver without running past its bounds: the driver trusts the
// lengths inside raw BSON, so a value Walk let through with a wrong one would panic here.
func FuzzWalk(f *testing.F) {
	f.Add(marshal(f, bson.D{{Key: "a", Value: bson.A{1, "x", bson.D{{Key: "b", Value: nil}}}}}))
	f.Add(marshal(f, bson.D{{Key: "c", Value: bson.CodeWithScope{Code: "f", Scope: bson.D{{Key: "d", Value: 1}}}}}))
	f.Add(marshal(f, bson.D{{Key: "e", Value: bson.Binary{Subtype: 0x02, Data: []byte{1}}}}))
	f.Add([]byte{0, 0, 0, 0})
	f.Add([]byte{12, 0, 0, 0, 0x03, 'a', 0, 0, 0, 0, 0, 0})

	f.Fuzz(func(t *testing.T, data []byte) {
		if bsonraw.Walk(bson.TypeEmbeddedDocument, data, 100, nil) != nil {
			return
		}

		if _, err := bson.Marshal(bson.D{{Key: "v", Value: bson.Raw(data)}}); err != nil {
			t.Fatalf("accepted raw BSON the driver refuses: %v", err)
		}
	})
}
