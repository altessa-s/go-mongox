// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-aip/decimal128"
	"github.com/altessa-s/go-atlas-core/types/optional"
)

// oversizedDecimal returns 1 written as 10^digits × 10^-digits: a value that fits a Decimal128 once its trailing zeros fold, and
// whose coefficient the driver would divide by ten digits times before writing it.
func oversizedDecimal(digits int64) decimal.Decimal {
	return decimal.NewFromBigInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(digits), nil), -int32(digits))
}

// decimalPositions returns d in every position a filter or a scope may hold a decimal.
func decimalPositions(d decimal.Decimal) map[string]any {
	return map[string]any{
		"decimal":               d,
		"valid null decimal":    decimal.NullDecimal{Decimal: d, Valid: true},
		"invalid null decimal":  decimal.NullDecimal{Decimal: d},
		"optional":              optional.Some(d),
		"optional null decimal": optional.Some(decimal.NullDecimal{Decimal: d, Valid: true}),
		"nested optional":       optional.Some(optional.Some(d)),
		"slice":                 []decimal.Decimal{decimal.Zero, d},
		"null decimal slice":    []decimal.NullDecimal{{Decimal: d, Valid: true}},
		"optional slice":        []optional.Optional[decimal.Decimal]{optional.Some(d)},
		"nested slice":          [][]decimal.Decimal{{decimal.Zero}, {d}},
		"array":                 bson.A{d},
		"operator":              bson.D{{Key: "amount", Value: bson.D{{Key: "$in", Value: []decimal.Decimal{d}}}}},
		"map":                   bson.M{"amount": bson.M{"$gte": d}},
	}
}

// TestValidateBoundsDecimalCoefficients checks a decimal whose coefficient is beyond decimal128.MaxCoefficientBits fails in
// every position, fast, before anything encodes it.
func TestValidateBoundsDecimalCoefficients(t *testing.T) {
	t.Parallel()

	huge := oversizedDecimal(200_000)

	for name, v := range decimalPositions(huge) {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			start := time.Now()

			err := Validate(t.Context(), bson.D{{Key: "v", Value: v}}, nil)
			if !errors.Is(err, ErrTooLarge) {
				t.Fatalf("got %v, want %v", err, ErrTooLarge)
			}

			if !errors.Is(err, errDecimalTooLarge) {
				t.Fatalf("got %v, want %v", err, errDecimalTooLarge)
			}

			requireQuick(t, time.Since(start), fastFail)
		})
	}
}

// TestValidateAdmitsBoundedDecimals checks a decimal within decimal128.MaxCoefficientBits stays admitted in every position,
// trailing zeros included.
func TestValidateAdmitsBoundedDecimals(t *testing.T) {
	t.Parallel()

	atLimit := oversizedDecimal(decimal128.MaxCoefficientDigits - 1)
	if !decimal128.CoefficientWithinLimit(atLimit) {
		t.Fatal("expected true: decimal128.CoefficientWithinLimit(atLimit)")
	}

	for _, d := range []decimal.Decimal{decimal.Zero, decimal.RequireFromString("-12.5"), atLimit, {}} {
		for name, v := range decimalPositions(d) {
			if err := Validate(t.Context(), bson.D{{Key: "v", Value: v}}, nil); err != nil {
				t.Log(name)
				t.Fatal(err)
			}
		}
	}

	if err := Validate(t.Context(), bson.D{{Key: "v", Value: decimal.NullDecimal{}}}, nil); err != nil {
		t.Fatal(err)
	}

	if err := Validate(t.Context(), bson.D{{Key: "v", Value: optional.None[decimal.Decimal]()}}, nil); err != nil {
		t.Fatal(err)
	}
}
