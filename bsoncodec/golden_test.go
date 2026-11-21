// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsoncodec_test

import (
	"bytes"
	"errors"
	"math/big"
	"os"
	"testing"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-atlas-core/time/civil"
	"github.com/altessa-s/go-atlas-core/types/optional"

	"github.com/altessa-s/go-mongox/bsoncodec"
)

func TestLegacyCodecBytes(t *testing.T) {
	t.Parallel()

	d := decimal.RequireFromString("12.50")
	doc := bson.D{{Key: "cloud", Value: civilDate{2024, 2, 29}}, {Key: "cloudZero", Value: civilDate{}}, {Key: "atlas", Value: civil.Date{Year: 2024, Month: 2, Day: 29}}, {Key: "atlasZero", Value: civil.Date{}}, {Key: "some", Value: optional.Some(d)}, {Key: "none", Value: optional.None[int]()}, {Key: "ptr", Value: &d}, {Key: "null", Value: decimal.NullDecimal{}}, {Key: "i32", Value: int32(42)}, {Key: "i64", Value: int64(42)}}

	got, err := marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	want, err := os.ReadFile("../testdata/codecs.bson")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("BSON mismatch: got %x, want %x", got, want)
	}
}
func TestBudgetBeforeDecimalNormalization(t *testing.T) {
	t.Parallel()

	for _, exponent := range []int64{1233, 1234} {
		coeff := new(big.Int).Exp(big.NewInt(10), big.NewInt(exponent), nil)
		d := decimal.NewFromBigInt(coeff, -int32(exponent))

		_, err := marshal(bson.D{{Key: "v", Value: d}})
		if coeff.BitLen() <= 4096 && err != nil || coeff.BitLen() > 4096 && !errors.Is(err, bsoncodec.ErrDecimalOutOfRange) {
			t.Fatalf("%d bits: %v", coeff.BitLen(), err)
		}
	}
}
