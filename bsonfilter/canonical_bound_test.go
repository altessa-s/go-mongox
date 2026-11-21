// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// prefixedMap returns a two-entry map whose keys share a prefix of n bytes.
func prefixedMap(n int) map[string]any {
	prefix := strings.Repeat("k", n)

	return map[string]any{prefix + "a": int32(1), prefix + "b": int32(2)}
}

// TestValidateBoundsSharedKeyBytes checks a filter referencing one map of long keys many times, small in memory and in
// values but huge in the key bytes its encoding and its canonicalization would go through, fails fast in the walk and in
// FindPage, before either compares a key.
func TestValidateBoundsSharedKeyBytes(t *testing.T) {
	t.Parallel()

	m := prefixedMap(1 << 20)

	refs := make(bson.A, 1<<16)
	for i := range refs {
		refs[i] = m
	}

	filter := bson.D{{Key: "x", Value: refs}}
	start := time.Now()

	if got := Validate(t.Context(), filter, nil); !errors.Is(got, ErrTooLarge) {
		t.Fatalf("got %v, want %v", got, ErrTooLarge)
	}

	_, err := Prepare(t.Context(), filter, nil, nil)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v, want %v", err, ErrTooLarge)
	}

	requireQuick(t, time.Since(start), fastFail)
}

// TestValidateCountsVariableBytes checks every key and every variable-size leaf counts against the byte budget, as often as
// the encoding repeats it, exactly at the limit.
func TestValidateCountsVariableBytes(t *testing.T) {
	t.Parallel()

	const fits = maxFilterBytes - 1 // the key "x" of the filter document

	half := func(n int) string { return strings.Repeat("v", n/2) }
	raw := func(n int) bson.Raw {
		r := rawOf(t, bson.D{{Key: "s", Value: strings.Repeat("r", n-13)}}) // the document, the key and the string framing
		if len(r) != n {
			t.Fatal("Len assertion failed for r")
		}

		return r
	}

	for name, sized := range map[string]func(n int) any{
		"string":        func(n int) any { return strings.Repeat("v", n) },
		"bytes":         func(n int) any { return make([]byte, n) },
		"binary":        func(n int) any { return bson.Binary{Data: make([]byte, n)} },
		"regex":         func(n int) any { return bson.Regex{Pattern: strings.Repeat("p", n-1), Options: "i"} },
		"key":           func(n int) any { return bson.D{{Key: strings.Repeat("k", n), Value: int32(1)}} },
		"map key":       func(n int) any { return bson.M{strings.Repeat("k", n): int32(1)} },
		"string list":   func(n int) any { return []string{half(n), half(n + 1)} },
		"shared string": func(n int) any { s := half(n); return bson.A{s, s + strings.Repeat("v", n%2)} },
		"raw":           func(n int) any { return raw(n) },
	} {
		if err := Validate(t.Context(), bson.D{{Key: "x", Value: sized(fits)}}, nil); err != nil {
			t.Log(name)
			t.Fatal(err)
		}

		if got := Validate(t.Context(), bson.D{{Key: "x", Value: sized(fits + 1)}}, nil); !errors.Is(got, ErrTooLarge) {
			t.Log(name)
			t.Fatalf("got %v, want %v", got, ErrTooLarge)
		}
	}

	shared := strings.Repeat("v", fits/2+1)
	if got := Validate(t.Context(), bson.D{{Key: "x", Value: bson.A{shared, shared}}}, nil); !errors.Is(got, ErrTooLarge) {
		t.Log("a shared string counts at every reference")
		t.Fatalf("got %v, want %v", got, ErrTooLarge)
	}
}

// TestCanonicalFilterCanonicalizesSharedMapsOnce checks a map reached through many references is canonicalized once: every
// reference gets the same canonical document, and a filter within the budget whose shared map has long keys canonicalizes fast.
func TestCanonicalFilterCanonicalizesSharedMapsOnce(t *testing.T) {
	t.Parallel()

	m := prefixedMap(1 << 16)

	refs := make(bson.A, 100)
	for i := range refs {
		refs[i] = m
	}

	filter := bson.D{{Key: "x", Value: refs}, {Key: "y", Value: []map[string]any{m}}}
	if err := Validate(t.Context(), filter, nil); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	canonical := mustCanonical(t, filter)

	requireQuick(t, time.Since(start), fastFail)

	list, ok := canonical[0].Value.(bson.A)
	if !ok {
		t.Fatal("expected true: ok")
	}

	first, ok := list[0].(bson.D)
	if !ok {
		t.Fatal("expected true: ok")
	}

	if len(first) != 2 {
		t.Fatal("Len assertion failed for first")
	}

	for _, v := range list {
		d, ok := v.(bson.D)
		if !ok {
			t.Fatal("expected true: ok")
		}

		if &first[0] != &d[0] {
			t.Log("every reference shares one canonical document")
			t.Fatal("Same assertion failed for &first[0]")
		}
	}

	requireSameBSON(t, filter, canonical)
}

// TestCanonicalFilterStopsOnCancel checks canonicalization returns the context's error once its context ends, and completes a
// pass shorter than one check interval.
func TestCanonicalFilterStopsOnCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	maps := make([]bson.M, 4*cancelCheckInterval)
	for i := range maps {
		maps[i] = bson.M{"a": int32(i), "b": int32(i)}
	}

	_, err := canonicalFilter(ctx, bson.D{{Key: "x", Value: maps}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want %v", err, context.Canceled)
	}

	_, err = Prepare(ctx, bson.D{{Key: "x", Value: maps}}, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want %v", err, context.Canceled)
	}

	got, err := canonicalFilter(ctx, bson.D{{Key: "x", Value: bson.M{"b": 1, "a": 2}}})
	if err != nil {
		t.Log("a pass shorter than one interval completes")
		t.Fatal(err)
	}

	if want, got := (bson.D{{Key: "x", Value: bson.D{{Key: "a", Value: 2}, {Key: "b", Value: 1}}}}), got; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
