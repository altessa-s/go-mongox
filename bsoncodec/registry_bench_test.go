// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsoncodec_test

import (
	"bytes"
	"testing"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-atlas-core/types/optional"

	"github.com/altessa-s/go-mongox/bsoncodec"
)

func BenchmarkOptionalEncode(b *testing.B) {
	var out bytes.Buffer

	enc := bson.NewEncoder(bson.NewDocumentWriter(&out))
	enc.SetRegistry(testRegistry())

	value := struct {
		Value optional.Optional[int] `bson:"value"`
	}{optional.Some(42)}

	for b.Loop() {
		out.Reset()

		if err := enc.Encode(value); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecimalRoundTrip(b *testing.B) {
	type doc struct {
		Amount decimal.Decimal `bson:"amount"`
	}

	benchRoundTrip(b, doc{Amount: decimal.RequireFromString("12345.6789")}, new(doc))
}

func BenchmarkCivilDateRoundTrip(b *testing.B) {
	type doc struct {
		Day civilDate `bson:"day"`
	}

	benchRoundTrip(b, doc{Day: civilDate{Year: 2026, Month: 10, Day: 9}}, new(doc))
}

// benchRoundTrip encodes in and decodes it into out with one registry built before the timer starts, so the codecs are
// measured rather than the registry construction.
func benchRoundTrip(b *testing.B, in, out any) {
	b.Helper()

	reg := testRegistry()

	var buf bytes.Buffer

	b.ReportAllocs()

	for b.Loop() {
		buf.Reset()

		enc := bson.NewEncoder(bson.NewDocumentWriter(&buf))
		enc.SetRegistry(reg)

		if err := enc.Encode(in); err != nil {
			b.Fatal(err)
		}

		dec := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(buf.Bytes())))
		dec.SetRegistry(reg)

		if err := dec.Decode(out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNewRegistry(b *testing.B) {
	for b.Loop() {
		_ = bsoncodec.NewRegistry()
	}
}
