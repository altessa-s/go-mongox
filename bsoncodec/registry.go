// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsoncodec

import (
	"reflect"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-atlas-core/types/optional"
)

// NewRegistry returns a private BSON registry with the driver's defaults and the codecs of the values go-aip and go-atlas-core
// define: every optional.Optional[T] (its value, or null), decimal.Decimal and decimal.NullDecimal (exact Decimal128) and the
// go-atlas-core civil.Date (a "YYYY-MM-DD" string). Install it on the client (options.Client().SetRegistry) so documents are
// stored with the same codecs as filters. Configure additional codecs with [RegisterDateType] before concurrent use.
//
// Null decodes into an absent Optional[T], a nil pointer, an invalid decimal.NullDecimal — and into a decimal.Decimal as zero,
// which cannot tell null from 0: declare a field that may hold null as decimal.NullDecimal, *decimal.Decimal or
// optional.Optional[decimal.Decimal].
func NewRegistry() *bson.Registry {
	defaults := bson.NewRegistry()
	// Obtain the standard structural hooks from a separate registry so their
	// lookup cache cannot shadow our replacements in the returned registry.
	enc, _ := defaults.LookupEncoder(reflect.TypeFor[optional.Optional[int]]())
	dec, _ := defaults.LookupDecoder(reflect.TypeFor[optional.Optional[int]]())
	reg := bson.NewRegistry()
	reg.RegisterInterfaceEncoder(reflect.TypeFor[bson.ValueMarshaler](), optionalCodecEncoder(enc))
	reg.RegisterInterfaceDecoder(reflect.TypeFor[bson.ValueUnmarshaler](), optionalCodecDecoder(dec))
	registerDecimal(reg)
	registerCivil(reg)

	return reg
}

// optionalCodecEncoder encodes an Optional[T] (or a pointer to one) as its inner
// value, or null when absent, and hands every other ValueMarshaler to the
// standard hook enc.
func optionalCodecEncoder(enc bson.ValueEncoder) bson.ValueEncoderFunc {
	return func(ec bson.EncodeContext, vw bson.ValueWriter, value reflect.Value) error {
		v := value
		if v.Kind() == reflect.Pointer && optional.IsOptionalType(v.Type().Elem()) {
			if v.IsNil() {
				return vw.WriteNull()
			}

			v = v.Elem()
		}

		if !optional.IsOptionalType(v.Type()) {
			return enc.EncodeValue(ec, vw, value)
		}

		inner, present := optional.GetReflect(v)
		if !present {
			return vw.WriteNull()
		}

		encoder, err := ec.LookupEncoder(inner.Type())
		if err != nil {
			return err
		}

		return encoder.EncodeValue(ec, vw, inner)
	}
}

// optionalCodecDecoder decodes null into an absent Optional[T] and any other value
// into a present one, and hands every other ValueUnmarshaler to the standard hook
// dec. A *Optional[T] mirrors the encoder: null sets it to nil, any other value
// decodes into the Optional it points to, allocated when nil.
func optionalCodecDecoder(dec bson.ValueDecoder) bson.ValueDecoderFunc {
	return func(dc bson.DecodeContext, vr bson.ValueReader, value reflect.Value) error {
		v := value
		if v.Kind() == reflect.Pointer && optional.IsOptionalType(v.Type().Elem()) {
			if vr.Type() == bson.TypeNull {
				if err := vr.ReadNull(); err != nil {
					return err
				}

				v.SetZero()

				return nil
			}

			if v.IsNil() {
				v.Set(reflect.New(v.Type().Elem()))
			}

			v = v.Elem()
		}

		if !optional.IsOptionalType(v.Type()) {
			return dec.DecodeValue(dc, vr, value)
		}

		if vr.Type() == bson.TypeNull {
			if err := vr.ReadNull(); err != nil {
				return err
			}

			v.Set(optional.NoneReflect(v.Type()))

			return nil
		}

		inner := reflect.New(optional.InnerType(v.Type())).Elem()

		decoder, err := dc.LookupDecoder(inner.Type())
		if err != nil {
			return err
		}

		if err := decoder.DecodeValue(dc, vr, inner); err != nil {
			return err
		}

		v.Set(optional.SomeReflect(v.Type(), inner))

		return nil
	}
}
