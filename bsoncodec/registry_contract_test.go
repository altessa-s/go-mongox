// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsoncodec_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-atlas-core/types/optional"

	"github.com/altessa-s/go-mongox/bsoncodec"
)

func TestCodecTypeMismatch(t *testing.T) {
	t.Parallel()

	reg := testRegistry()
	for _, typ := range []reflect.Type{reflect.TypeFor[civilDate](), reflect.TypeFor[decimal.Decimal](), reflect.TypeFor[decimal.NullDecimal](), reflect.TypeFor[*decimal.Decimal](), reflect.TypeFor[*decimal.NullDecimal]()} {
		enc, err := reg.LookupEncoder(typ)
		if err != nil {
			t.Fatal(err)
		}

		err = enc.EncodeValue(bson.EncodeContext{Registry: reg}, nil, reflect.ValueOf("wrong type"))

		if _, ok := errors.AsType[bson.ValueEncoderError](err); !ok {
			t.Fatalf("%v encoder: %v", typ, err)
		}

		dec, err := reg.LookupDecoder(typ)
		if err != nil {
			t.Fatal(err)
		}

		err = dec.DecodeValue(bson.DecodeContext{Registry: reg}, nil, reflect.ValueOf("wrong type"))

		if _, ok := errors.AsType[bson.ValueDecoderError](err); !ok {
			t.Fatalf("%v decoder: %v", typ, err)
		}
	}
}

func TestPointerToOptional(t *testing.T) {
	t.Parallel()

	type record struct {
		Value *optional.Optional[int] `bson:"v"`
	}

	value := optional.Some(17)
	for _, in := range []record{{&value}, {}} {
		raw, err := marshal(in)
		if err != nil {
			t.Fatal(err)
		}

		var out record
		if err := unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}

		if in.Value != nil && (out.Value == nil || *out.Value != *in.Value) {
			t.Fatal("present value lost")
		}

		if in.Value == nil && bson.Raw(raw).Lookup("v").Type != bson.TypeNull {
			t.Fatal("nil pointer must encode as null")
		}
	}
}

func TestOptionalWithUnsupportedInnerType(t *testing.T) {
	t.Parallel()

	reg := bsoncodec.NewRegistry()

	var buf bytes.Buffer

	enc := bson.NewEncoder(bson.NewDocumentWriter(&buf))
	enc.SetRegistry(reg)

	doc := struct {
		Value optional.Optional[chan int] `bson:"v"`
	}{optional.Some(make(chan int))}
	if err := enc.Encode(doc); err == nil {
		t.Fatalf("missing inner encoder: %v", err)
	}

	raw, err := bson.Marshal(bson.D{{Key: "v", Value: 1}})
	if err != nil {
		t.Fatal(err)
	}

	if err := unmarshal(raw, &doc); err == nil {
		t.Fatalf("missing inner decoder: %v", err)
	}
}
