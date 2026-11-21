// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-aip/decimal128"

	filtereval "github.com/altessa-s/go-aip/filter/evaluator"
	coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
)

// maxFilterNesting bounds the nesting [Validate] walks; a deeper or self-referential value fails closed.
const maxFilterNesting = 1000

// maxFilterNodes bounds the values [Validate] visits in one filter, counted as the encoding would repeat them: a value
// sharing its parts (a list holding another twice, nested) is as large as its expansion, which a small graph makes exponential
// in its depth.
const maxFilterNodes = 1 << 18

// maxFilterBytes bounds the BSON a filter encodes to, the raw BSON a filter may hold and the variable-size bytes [Validate]
// counts (keys, strings, binaries, regular expressions and raw BSON, each as often as the encoding repeats it): MongoDB's
// document limit. A filter within it is bounded for every pass over it, canonicalization included, not only for the encoder.
const maxFilterBytes = filtereval.MaxDocumentBytes

// encodeNesting bounds the documents the bounded encoder of a filter holds open at once: [maxFilterNesting] and the wrapping
// document.
const encodeNesting = maxFilterNesting + 1

// cancelCheckInterval is the number of visits between two checks of the context of a [Validate] walk.
const cancelCheckInterval = 1024

var (
	// ErrDangerousOperator identifies a filter containing a server-side JavaScript operator.
	ErrDangerousOperator = errors.New("mongo: filter contains a dangerous MongoDB operator")
	// ErrValueType identifies a filter value whose Go type, or raw BSON type, is not one [Validate] admits.
	ErrValueType = errors.New("mongo: filter value type is not admitted")
)

// ErrTooDeep fails a filter nested beyond [maxFilterNesting].
var ErrTooDeep = errors.New("mongo: filter nesting exceeds validation limit")

// ErrTooLarge fails a filter larger than [maxFilterNodes] values or [maxFilterBytes] variable-size bytes, expanded as the
// encoding repeats them, or holding raw BSON larger than [maxFilterBytes].
var ErrTooLarge = errors.New("mongo: filter size exceeds validation limit")

// errDecimalTooLarge fails a decimal whose coefficient is beyond [decimal128.MaxCoefficientBits] bits: encoding it would take time
// proportional to its size before any byte is written, which the encoded size bound cannot stop.
var errDecimalTooLarge = fmt.Errorf("%w: decimal coefficient beyond %d bits", ErrTooLarge, decimal128.MaxCoefficientBits)

// ErrMalformedRaw fails raw BSON that is not well formed.
var ErrMalformedRaw = errors.New("mongo: filter holds malformed raw BSON")

// dangerousFilterOperators is the deny-list of MongoDB query operators
// that execute server-side JavaScript (or otherwise have a well-known
// abuse path) and therefore must never appear in a filter built from
// untrusted input. The list is intentionally narrow — perf-only hazards
// like $regex stay allowed; only operators that grant attacker-controlled
// code execution on the database are blocked.
var dangerousFilterOperators = coremaps.NewImmutableMap(map[string]struct{}{
	"$where":       {}, // arbitrary JavaScript evaluated server-side
	"$function":    {}, // user-defined JavaScript inside aggregations / $expr
	"$accumulator": {}, // user-defined JavaScript accumulator
})

// Validate checks a filter, or one value of a filter, before it is encoded. It admits the containers the filter
// translator, the list engine and an application build filters from — bson.D, bson.E, bson.M, map[string]any, bson.A, []any and
// any other unnamed slice of admitted values ([]bson.D, []bson.M, []string, …) — raw BSON (bson.Raw, bson.RawArray,
// bson.RawValue) and the leaf types of [isLeaf]; any other value (a struct, a pointer, another map or a named slice, a type with
// its own BSON marshaling, a channel, a function) fails with [ErrValueType]. Values reg encodes as scalars ([AdmitScalar])
// are leaves too; reg may be nil. A decimal — decimal.Decimal, decimal.NullDecimal, an optional.Optional of one, alone or in a
// list — whose coefficient is beyond [decimal128.MaxCoefficientBits] bits fails before it is encoded, in time independent of its size.
//
// Every key at every depth, raw BSON included, is checked against the deny-list (see [dangerousFilterOperators]), which fails
// with [ErrDangerousOperator]. The walk is bounded: it fails past [maxFilterNesting] levels, past [maxFilterNodes] values and
// past [maxFilterBytes] bytes of keys, strings, binaries, regular expressions and raw BSON — every container element and every
// key counts, and a value reached again through a shared reference counts again, as the driver encodes it again — refuses raw
// BSON beyond [maxFilterBytes] before reading it, and stops with the context's error once ctx ends.
// Query helpers that accept filters from untrusted boundaries MUST run this before passing the filter to the driver.
func Validate(ctx context.Context, filter any, reg *bson.Registry) error {
	w := newFilterWalk(ctx, reg)

	return w.value(filter, 0)
}

// filterWalk is one bounded [Validate] walk: the visits and bytes left, the context it stops with and the registry whose
// scalar types it admits.
type filterWalk struct {
	ctx        context.Context //nolint:containedctx // the walk lives for one call
	reg        *bson.Registry
	left       int
	bytesLeft  int
	untilCheck int
}

// newFilterWalk returns a walk with the whole budget.
func newFilterWalk(ctx context.Context, reg *bson.Registry) filterWalk {
	return filterWalk{ctx: ctx, reg: reg, left: maxFilterNodes, bytesLeft: maxFilterBytes, untilCheck: cancelCheckInterval}
}

// spend charges n variable-size bytes: it fails once more than [maxFilterBytes] were charged.
func (w *filterWalk) spend(n int) error {
	if n > w.bytesLeft {
		w.bytesLeft = 0

		return ErrTooLarge
	}

	w.bytesLeft -= n

	return nil
}

// take spends n visits: it fails once more than [maxFilterNodes] were spent, and checks the context every
// [cancelCheckInterval] visits.
func (w *filterWalk) take(n int) error {
	if n > w.left {
		w.left = 0

		return ErrTooLarge
	}

	w.left -= n

	if w.untilCheck -= n; w.untilCheck <= 0 {
		w.untilCheck = cancelCheckInterval

		if w.ctx != nil {
			if err := w.ctx.Err(); err != nil {
				return err
			}
		}
	}

	return nil
}

// key fails for a deny-listed key.
func key(k string) error {
	if dangerousFilterOperators.Contains(k) {
		return fmt.Errorf("%w: %s", ErrDangerousOperator, k)
	}

	return nil
}

// value visits v, depth levels deep: one visit for v itself, then its elements.
func (w *filterWalk) value(v any, depth int) error {
	if depth > maxFilterNesting {
		return ErrTooDeep
	}

	if err := w.take(1); err != nil {
		return err
	}

	switch v := v.(type) {
	case nil:
		return nil
	case bson.D:
		for _, e := range v {
			if err := w.element(e, depth+1); err != nil {
				return err
			}
		}

		return nil
	case bson.E:
		return w.element(v, depth+1)
	case bson.M:
		return document(w, v, depth+1)
	case map[string]any:
		return document(w, v, depth+1)
	case bson.A:
		return list(w, v, depth+1)
	case []any:
		return list(w, v, depth+1)
	case []bson.D:
		return list(w, v, depth+1)
	case []bson.M:
		return list(w, v, depth+1)
	case bson.Raw:
		return w.raw(v, func() error { return w.rawDocument(v, depth+1) })
	case bson.RawArray:
		return w.raw(v, func() error { return w.rawDocument(v, depth+1) })
	case bson.RawValue:
		return w.raw(v.Value, func() error { return w.rawValue(v.Type, v.Value, depth+1) })
	default:
		return w.other(reflect.ValueOf(v), depth+1)
	}
}

// raw charges the bytes of raw BSON, refusing more than [maxFilterBytes] before reading any, and then walks it.
func (w *filterWalk) raw(b []byte, walk func() error) error {
	if len(b) > maxFilterBytes {
		return ErrTooLarge
	}

	if err := w.spend(len(b)); err != nil {
		return err
	}

	return walk()
}

// element checks and charges the key of e and visits its value.
func (w *filterWalk) element(e bson.E, depth int) error {
	if err := w.key(e.Key); err != nil {
		return err
	}

	return w.value(e.Value, depth)
}

// key fails for a deny-listed key and charges its bytes.
func (w *filterWalk) key(k string) error {
	if err := key(k); err != nil {
		return err
	}

	return w.spend(len(k))
}

// document checks and charges every key of m and visits its values.
func document[M ~map[string]any](w *filterWalk, m M, depth int) error {
	for k, v := range m {
		if err := w.key(k); err != nil {
			return err
		}

		if err := w.value(v, depth); err != nil {
			return err
		}
	}

	return nil
}

// list visits every element of s.
func list[S ~[]E, E any](w *filterWalk, s S, depth int) error {
	for _, v := range s {
		if err := w.value(v, depth); err != nil {
			return err
		}
	}

	return nil
}

// other visits a value of no type [filterWalk.value] lists: an admitted leaf, or an unnamed slice of admitted values; anything
// else fails with [ErrValueType].
func (w *filterWalk) other(v reflect.Value, depth int) error {
	t := v.Type()

	switch {
	case isLeaf(t, w.reg):
		if err := w.spend(leafBytes(v)); err != nil {
			return err
		}

		return checkLeaf(v)
	case !isList(t, w.reg):
		return fmt.Errorf("%w: %s", ErrValueType, t)
	case isLeaf(t.Elem(), w.reg) && !inspected(t.Elem()): // every element is a plain leaf: charged, not visited one by one
		if err := w.take(v.Len()); err != nil {
			return err
		}

		return w.spendLeaves(v)
	}

	for i := range v.Len() {
		if err := w.value(v.Index(i).Interface(), depth); err != nil {
			return err
		}
	}

	return nil
}
