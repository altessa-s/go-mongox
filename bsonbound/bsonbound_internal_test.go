// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonbound

import (
	"bytes"
	"errors"
	"strconv"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	filtereval "github.com/altessa-s/go-aip/filter/evaluator"
)

// testNesting is the nesting limit of the tests.
const testNesting = 102

// encodeBounded encodes doc through a bounded writer of the given limit.
func encodeBounded(tb testing.TB, doc any, limit int) (*bytes.Buffer, *Bound, error) {
	tb.Helper()

	var buf bytes.Buffer

	vw, bound := NewValueWriter(bson.NewDocumentWriter(&buf), limit, testNesting, nil)
	err := bson.NewEncoder(vw).Encode(doc)

	return &buf, bound, err
}

func writerCorpus() bson.D {
	return bson.D{
		{Key: "string", Value: "text"},
		{Key: "int32", Value: int32(1)},
		{Key: "int64", Value: int64(2)},
		{Key: "double", Value: 1.5},
		{Key: "bool", Value: true},
		{Key: "binary", Value: bson.Binary{Data: []byte{1, 2, 3}}},
		{Key: "old", Value: bson.Binary{Subtype: bson.TypeBinaryBinaryOld, Data: []byte{1, 2}}},
		{Key: "date", Value: bson.DateTime(3)},
		{Key: "decimal", Value: bson.NewDecimal128(1, 2)},
		{Key: "oid", Value: bson.ObjectID{1}},
		{Key: "regex", Value: bson.Regex{Pattern: "a+", Options: "i"}},
		{Key: "js", Value: bson.JavaScript("f()")},
		{Key: "symbol", Value: bson.Symbol("s")},
		{Key: "ts", Value: bson.Timestamp{T: 1, I: 2}},
		{Key: "pointer", Value: bson.DBPointer{DB: "db.c", Pointer: bson.ObjectID{2}}},
		{Key: "null", Value: nil},
		{Key: "min", Value: bson.MinKey{}},
		{Key: "max", Value: bson.MaxKey{}},
		{Key: "undefined", Value: bson.Undefined{}},
		{Key: "doc", Value: bson.D{{Key: "a", Value: bson.A{1, "b", bson.D{}}}}},
		{Key: "array", Value: bson.A{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
		{Key: "scope", Value: bson.CodeWithScope{Code: "g()", Scope: bson.D{{Key: "x", Value: int32(1)}}}},
	}
}

// Every prefix limit smaller than a single element's BSON must fail,
// including limits inside document headers, keys and variable-size values.
func TestBoundedWriterElementLimits(t *testing.T) {
	t.Parallel()

	for _, e := range writerCorpus() {
		t.Run(e.Key, func(t *testing.T) {
			doc := bson.D{e}

			want, err := bson.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}

			for limit := range len(want) {
				_, _, err := encodeBounded(t, doc, limit)
				if !errors.Is(err, ErrLimitExceeded) {
					t.Fatalf("limit %d: %v", limit, err)
				}
			}
		})
	}
}

func TestBoundedWriterMeasuresExactSize(t *testing.T) {
	t.Parallel()

	doc := writerCorpus()

	buf, bound, err := encodeBounded(t, doc, filtereval.MaxDocumentBytes)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := buf.Len(), bound.size; want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if bound.depth != 0 {
		t.Fatal("expected zero: bound.depth")
	}

	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	// A bson.Raw is copied value by value through the bounded writer.
	buf, bound, err = encodeBounded(t, bson.D{{Key: "raw", Value: bson.Raw(raw)}}, filtereval.MaxDocumentBytes)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := buf.Len(), bound.size; want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestBoundedWriterRefusesPastLimit(t *testing.T) {
	t.Parallel()

	doc := bson.D{{Key: "v", Value: "0123456789"}}

	buf, _, err := encodeBounded(t, doc, filtereval.MaxDocumentBytes)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = encodeBounded(t, doc, buf.Len())
	if err != nil {
		t.Log("an encoding of exactly the limit is admitted")
		t.Fatal(err)
	}

	_, _, err = encodeBounded(t, doc, buf.Len()-1)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("got %v, want %v", err, ErrLimitExceeded)
	}
}

func TestBoundedWriterRefusesDeepNesting(t *testing.T) {
	t.Parallel()

	var deep any = int32(1)
	for range testNesting {
		deep = bson.D{{Key: "d", Value: deep}}
	}

	_, _, err := encodeBounded(t, bson.D{{Key: "v", Value: deep}}, filtereval.MaxDocumentBytes)
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("got %v, want %v", err, ErrLimitExceeded)
	}
}

func TestDecimalDigits(t *testing.T) {
	t.Parallel()

	for _, n := range []int{0, 1, 9, 10, 99, 100, 999, 1000, 99_999, 100_000, 1<<31 - 1} {
		if got, want := decimalDigits(n), len(strconv.Itoa(n)); got != want {
			t.Fatalf("decimalDigits(%d) = %d, want %d", n, got, want)
		}
	}
}

// TestLargeArraySize checks that the bound counts the keys of an array with indexes of several digits as the driver writes them.
func TestLargeArraySize(t *testing.T) {
	t.Parallel()

	values := make(bson.A, 1234)
	for i := range values {
		values[i] = int32(i)
	}

	doc := bson.D{{Key: "a", Value: values}}

	buf, bound, err := encodeBounded(t, doc, 1<<20)
	if err != nil {
		t.Fatal(err)
	}

	if bound.Size() != buf.Len() {
		t.Fatalf("bound size %d, encoded %d bytes", bound.Size(), buf.Len())
	}
}
