// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"reflect"
	"runtime"
	"sync"
	"time"
	"weak"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-aip/decimal128"
	"github.com/altessa-s/go-atlas-core/types/optional"

	coremaps "github.com/altessa-s/go-atlas-core/collections/maps"
)

// leafTypes are the struct and slice types [Validate] admits as leaves: values whose encoding holds no key, no other
// value and no reference.
var leafTypes = coremaps.NewImmutableMap(map[reflect.Type]struct{}{
	reflect.TypeFor[[]byte]():              {},
	reflect.TypeFor[time.Time]():           {},
	reflect.TypeFor[bson.DateTime]():       {},
	reflect.TypeFor[bson.Timestamp]():      {},
	reflect.TypeFor[bson.ObjectID]():       {},
	reflect.TypeFor[bson.Decimal128]():     {},
	reflect.TypeFor[bson.Regex]():          {},
	reflect.TypeFor[bson.Binary]():         {},
	reflect.TypeFor[bson.Null]():           {},
	reflect.TypeFor[bson.Undefined]():      {},
	reflect.TypeFor[bson.MinKey]():         {},
	reflect.TypeFor[bson.MaxKey]():         {},
	reflect.TypeFor[decimal.Decimal]():     {}, // its coefficient is bounded ([checkLeaf])
	reflect.TypeFor[decimal.NullDecimal](): {}, // its coefficient is bounded ([checkLeaf])
})

var (
	decimalType     = reflect.TypeFor[decimal.Decimal]()
	nullDecimalType = reflect.TypeFor[decimal.NullDecimal]()
)

// inspected reports whether a leaf of type t is inspected before it is encoded ([checkLeaf]): a decimal, or an optional.Optional
// of one.
func inspected(t reflect.Type) bool {
	switch {
	case t == decimalType, t == nullDecimalType:
		return true
	case t.Kind() == reflect.Struct && optional.IsOptionalType(t):
		return inspected(optional.InnerType(t))
	default:
		return false
	}
}

// checkLeaf fails a leaf the encoding cannot bound: a decimal, valid or not, whose coefficient is beyond
// [decimal128.MaxCoefficientBits] bits, which the driver would divide by ten digit by digit before writing it. The check neither copies
// nor formats the coefficient.
func checkLeaf(v reflect.Value) error {
	switch t := v.Type(); {
	case t == decimalType:
		d, _ := reflect.TypeAssert[decimal.Decimal](v)

		return checkDecimal(d)
	case t == nullDecimalType:
		nd, _ := reflect.TypeAssert[decimal.NullDecimal](v)

		return checkDecimal(nd.Decimal)
	case t.Kind() == reflect.Struct && optional.IsOptionalType(t) && inspected(t):
		inner, _ := optional.GetReflect(v)

		return checkLeaf(inner)
	default:
		return nil
	}
}

// checkDecimal fails d when its coefficient is beyond [decimal128.MaxCoefficientBits] bits ([decimal128.CoefficientWithinLimit]).
func checkDecimal(d decimal.Decimal) error {
	if !decimal128.CoefficientWithinLimit(d) {
		return errDecimalTooLarge
	}

	return nil
}

var (
	bytesType  = reflect.TypeFor[[]byte]()
	binaryType = reflect.TypeFor[bson.Binary]()
	regexType  = reflect.TypeFor[bson.Regex]()
)

// leafBytes returns the variable-size bytes of the leaf v the encoding writes: the bytes of a string, a []byte, the data of a
// binary and the pattern and options of a regular expression, also inside an optional.Optional; 0 for a fixed-size leaf.
func leafBytes(v reflect.Value) int {
	switch t := v.Type(); {
	case t.Kind() == reflect.String, t == bytesType:
		return v.Len()
	case t == binaryType:
		b, _ := reflect.TypeAssert[bson.Binary](v)

		return len(b.Data)
	case t == regexType:
		r, _ := reflect.TypeAssert[bson.Regex](v)

		return len(r.Pattern) + len(r.Options)
	case t.Kind() == reflect.Struct && optional.IsOptionalType(t):
		if inner, ok := optional.GetReflect(v); ok {
			return leafBytes(inner)
		}

		return 0
	default:
		return 0
	}
}

// sizedLeaf reports whether a leaf of type t may have variable-size bytes ([leafBytes]).
func sizedLeaf(t reflect.Type) bool {
	switch {
	case t.Kind() == reflect.String, t == bytesType, t == binaryType, t == regexType:
		return true
	case t.Kind() == reflect.Struct && optional.IsOptionalType(t):
		return sizedLeaf(optional.InnerType(t))
	default:
		return false
	}
}

// spendLeaves charges the variable-size bytes of every element of v, a list of plain leaves.
func (w *filterWalk) spendLeaves(v reflect.Value) error {
	if !sizedLeaf(v.Type().Elem()) {
		return nil
	}

	for i := range v.Len() {
		if err := w.spend(leafBytes(v.Index(i))); err != nil {
			return err
		}
	}

	return nil
}

// containerTypes are the named container types [Validate] walks.
var containerTypes = coremaps.NewImmutableMap(map[reflect.Type]struct{}{
	reflect.TypeFor[bson.D]():         {},
	reflect.TypeFor[bson.E]():         {},
	reflect.TypeFor[bson.M]():         {},
	reflect.TypeFor[map[string]any](): {},
	reflect.TypeFor[bson.A]():         {},
	reflect.TypeFor[bson.Raw]():       {},
	reflect.TypeFor[bson.RawArray]():  {},
	reflect.TypeFor[bson.RawValue]():  {},
	reflect.TypeFor[any]():            {},
})

// codeTypes are the named strings the driver encodes as server-side code or as the deprecated symbol, not as strings.
var codeTypes = coremaps.NewImmutableMap(map[reflect.Type]struct{}{
	reflect.TypeFor[bson.JavaScript](): {},
	reflect.TypeFor[bson.Symbol]():     {},
})

var (
	marshalerType      = reflect.TypeFor[bson.Marshaler]()
	valueMarshalerType = reflect.TypeFor[bson.ValueMarshaler]()
)

// isLeaf reports whether [Validate] admits values of t as leaves: a boolean, a number or a string of a type without its own
// BSON marshaling (but not one of [codeTypes]), one of [leafTypes], an optional.Optional of a leaf, or a type reg encodes as a
// scalar ([AdmitScalar]).
func isLeaf(t reflect.Type, reg *bson.Registry) bool {
	if leafTypes.Contains(t) {
		return true
	}

	switch t.Kind() { //nolint:exhaustive // every other kind is a container or refused
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
		return !marshals(t) && !codeTypes.Contains(t)
	case reflect.Struct:
		if optional.IsOptionalType(t) {
			return isLeaf(optional.InnerType(t), reg)
		}

		return registered(reg, t)
	default:
		return false
	}
}

// isList reports whether t is an unnamed slice type whose elements [Validate] admits.
func isList(t reflect.Type, reg *bson.Registry) bool {
	if t.Kind() != reflect.Slice || t.Name() != "" {
		return false
	}

	e := t.Elem()

	return containerTypes.Contains(e) || isLeaf(e, reg) || isList(e, reg)
}

// marshals reports whether t or *t has its own BSON marshaling.
func marshals(t reflect.Type) bool {
	p := reflect.PointerTo(t)

	return t.Implements(marshalerType) || t.Implements(valueMarshalerType) || p.Implements(marshalerType) || p.Implements(valueMarshalerType)
}

// scalarTypes holds, per registry, the types [AdmitScalar] admitted for it: a *sync.Map of reflect.Type keyed by a weak
// pointer to the registry, so an entry does not keep its registry alive and goes when the registry does.
var scalarTypes sync.Map

// AdmitScalar makes [Validate] admit values of t as leaves for reg: reg has a codec that encodes every value of t as a
// scalar, such as the calendar-date codec of bsoncodec.RegisterDateType. It is safe for concurrent use.
func AdmitScalar(reg *bson.Registry, t reflect.Type) {
	if reg == nil || t == nil || t.Kind() != reflect.Struct {
		return
	}

	k := weak.Make(reg)

	set, loaded := scalarTypes.LoadOrStore(k, new(sync.Map))
	if !loaded {
		runtime.AddCleanup(reg, func(k weak.Pointer[bson.Registry]) { scalarTypes.Delete(k) }, k)
	}

	if types, ok := set.(*sync.Map); ok {
		types.Store(t, struct{}{})
	}
}

// registered reports whether t was admitted for reg with [AdmitScalar].
func registered(reg *bson.Registry, t reflect.Type) bool {
	if reg == nil {
		return false
	}

	set, ok := scalarTypes.Load(weak.Make(reg))
	if !ok {
		return false
	}

	types, ok := set.(*sync.Map)
	if !ok {
		return false
	}

	_, ok = types.Load(t)

	return ok
}
