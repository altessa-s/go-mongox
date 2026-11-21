// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsoncodec_test

import (
	"errors"
	"math/rand/v2"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-aip/decimal128"

	"github.com/altessa-s/go-mongox/bsoncodec"
)

type money struct {
	Amount   decimal.Decimal      `bson:"amount"`
	Fee      *decimal.Decimal     `bson:"fee"`
	Discount decimal.NullDecimal  `bson:"discount"`
	NullFee  *decimal.NullDecimal `bson:"null_fee"`
}

func TestDecimalRoundTrip(t *testing.T) {
	t.Parallel()

	for _, s := range []string{
		"0", "-0.01", "123.4500", "1e-6176", "9999999999999999999999999999999999",
		"12345678901234567890123456789012340000000", // trailing zeros fold into the exponent
		"-3.14159265358979323846264338327950",
	} {
		t.Run(s, func(t *testing.T) {
			t.Parallel()

			in := money{Amount: decimal.RequireFromString(s), Discount: decimal.NewNullDecimal(decimal.RequireFromString(s))}

			raw, err := marshal(in)
			if err != nil {
				t.Fatal(err)
			}

			if want, got := bson.TypeDecimal128, bson.Raw(raw).Lookup("amount").Type; !reflect.DeepEqual(want, got) {
				t.Log("stored as Decimal128")
				t.Fatalf("got %#v, want %#v", got, want)
			}

			if want, got := bson.TypeNull, bson.Raw(raw).Lookup("fee").Type; !reflect.DeepEqual(want, got) {
				t.Log("nil pointer is null")
				t.Fatalf("got %#v, want %#v", got, want)
			}

			if want, got := bson.TypeNull, bson.Raw(raw).Lookup("null_fee").Type; !reflect.DeepEqual(want, got) {
				t.Log("nil NullDecimal pointer is null")
				t.Fatalf("got %#v, want %#v", got, want)
			}

			var out money
			if err := unmarshal(raw, &out); err != nil {
				t.Fatal(err)
			}

			if !in.Amount.Equal(out.Amount) {
				t.Logf("%s != %s", in.Amount, out.Amount)
				t.Fatal("expected true: in.Amount.Equal(out.Amount)")
			}

			if out.Fee != nil {
				t.Fatal("expected nil: out.Fee")
			}

			if out.NullFee != nil {
				t.Fatal("expected nil: out.NullFee")
			}

			if !out.Discount.Valid {
				t.Fatal("expected true: out.Discount.Valid")
			}

			if !in.Amount.Equal(out.Discount.Decimal) {
				t.Fatal("expected true: in.Amount.Equal(out.Discount.Decimal)")
			}
		})
	}
}

func TestDecimalPointerAndNull(t *testing.T) {
	t.Parallel()

	fee := decimal.RequireFromString("1.25")
	nullFee := decimal.NewNullDecimal(decimal.RequireFromString("2.50"))

	raw, err := marshal(money{Fee: &fee, NullFee: &nullFee})
	if err != nil {
		t.Fatal(err)
	}

	if want, got := bson.TypeNull, bson.Raw(raw).Lookup("discount").Type; !reflect.DeepEqual(want, got) {
		t.Log("invalid NullDecimal is null")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	var out money
	if err := unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if out.Fee == nil {
		t.Fatal("expected non-nil: out.Fee")
	}

	if !fee.Equal(*out.Fee) {
		t.Fatal("expected true: fee.Equal(*out.Fee)")
	}

	if out.NullFee == nil {
		t.Fatal("expected non-nil: out.NullFee")
	}

	if !out.NullFee.Valid {
		t.Fatal("expected true: out.NullFee.Valid")
	}

	if !nullFee.Decimal.Equal(out.NullFee.Decimal) {
		t.Fatal("expected true: nullFee.Decimal.Equal(out.NullFee.Decimal)")
	}

	if out.Discount.Valid {
		t.Fatal("expected false: out.Discount.Valid")
	}
}

func TestDecimalPointerDecodeSources(t *testing.T) {
	t.Parallel()

	d128, err := bson.ParseDecimal128("12.50")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "double", value: 1.5, wantErr: true},
		{name: "string", value: "12.50", wantErr: true},
		{name: "Decimal128", value: d128},
		{name: "null", value: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			raw, err := bson.Marshal(bson.M{"decimal": tc.value, "null_decimal": tc.value})
			if err != nil {
				t.Fatal(err)
			}

			var out struct {
				Decimal     *decimal.Decimal     `bson:"decimal"`
				NullDecimal *decimal.NullDecimal `bson:"null_decimal"`
			}

			err = unmarshal(raw, &out)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected an error from err")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if tc.value == nil {
				if out.Decimal != nil {
					t.Fatal("expected nil: out.Decimal")
				}

				if out.NullDecimal != nil {
					t.Fatal("expected nil: out.NullDecimal")
				}

				return
			}

			if out.Decimal == nil {
				t.Fatal("expected non-nil: out.Decimal")
			}

			if !decimal.RequireFromString("12.5").Equal(*out.Decimal) {
				t.Fatal("expected true: decimal.RequireFromString(\"12.5\").Equal(*out.Decimal)")
			}

			if out.NullDecimal == nil {
				t.Fatal("expected non-nil: out.NullDecimal")
			}

			if !out.NullDecimal.Valid {
				t.Fatal("expected true: out.NullDecimal.Valid")
			}

			if !decimal.RequireFromString("12.5").Equal(out.NullDecimal.Decimal) {
				t.Fatal("expected true: decimal.RequireFromString(\"12.5\").Equal(out.NullDecimal.Decimal)")
			}
		})
	}
}

func TestDecimalOutOfRange(t *testing.T) {
	t.Parallel()

	for _, s := range []string{
		"1." + strings.Repeat("1", 34), // 35 significant digits
		"1e6145",                       // exponent above the Decimal128 range
		"1e-6177",                      // below it
	} {
		_, err := marshal(money{Amount: decimal.RequireFromString(s)})
		if !errors.Is(err, bsoncodec.ErrDecimalOutOfRange) {
			t.Log(s)
			t.Fatalf("got %v, want %v", err, bsoncodec.ErrDecimalOutOfRange)
		}
	}
}

// TestDecimalCodecMatchesFits pins decimal128.Fits to the encoder's verdict: the codec stores exactly the decimals it accepts.
func TestDecimalCodecMatchesFits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		d    decimal.Decimal
		want bool
	}{
		{name: "zero", d: decimal.Zero, want: true},
		{name: "negative cents", d: decimal.RequireFromString("-0.01"), want: true},
		{name: "34 digits", d: decimal.RequireFromString(strings.Repeat("9", 34)), want: true},
		{name: "35 digits", d: decimal.RequireFromString(strings.Repeat("9", 35)), want: false},
		{name: "35 digits with trailing zero", d: decimal.RequireFromString(strings.Repeat("9", 34) + "0"), want: true},
		{name: "35 significant fraction digits", d: decimal.RequireFromString("1." + strings.Repeat("1", 34)), want: false},
		{name: "min exponent", d: decimal.New(1, -6176), want: true},
		{name: "below min exponent", d: decimal.New(1, -6177), want: false},
		{name: "below min exponent folds", d: decimal.New(10, -6177), want: true},
		{name: "max exponent", d: decimal.New(1, 6111), want: true},
		{name: "above max exponent clamps", d: decimal.New(1, 6144), want: true},
		{name: "above max exponent too many digits", d: decimal.New(1, 6145), want: false},
		{name: "extreme negative exponent", d: decimal.New(1, -2147483648), want: false},
		{name: "extreme positive exponent", d: decimal.New(1, 2147483647), want: false},
		{name: "zero with extreme exponent", d: decimal.New(0, 7000)},
		{name: "zero with extreme negative exponent", d: decimal.New(0, -7000)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := decimal128.Fits(tc.d)
			_, err := marshal(money{Amount: tc.d})
			// The predicate is exactly the encoder's verdict.
			if want, got := err == nil, got; !reflect.DeepEqual(want, got) {
				t.Fatalf("got %#v, want %#v", got, want)
			}

			if err != nil {
				if !errors.Is(err, bsoncodec.ErrDecimalOutOfRange) {
					t.Fatalf("got %v, want %v", err, bsoncodec.ErrDecimalOutOfRange)
				}
			}

			if !strings.HasPrefix(tc.name, "zero with") {
				if want, got := tc.want, got; !reflect.DeepEqual(want, got) {
					t.Fatalf("got %#v, want %#v", got, want)
				}
			}
		})
	}
}

func TestDecimalExtremeExponentFailsFast(t *testing.T) {
	t.Parallel()

	// Formatting this value would allocate billions of zeros.
	_, err := marshal(money{Amount: decimal.New(1, -2147483648)})
	if !errors.Is(err, bsoncodec.ErrDecimalOutOfRange) {
		t.Fatalf("got %v, want %v", err, bsoncodec.ErrDecimalOutOfRange)
	}

	_, err = marshal(money{Amount: decimal.New(1, 2147483647)})
	if !errors.Is(err, bsoncodec.ErrDecimalOutOfRange) {
		t.Fatalf("got %v, want %v", err, bsoncodec.ErrDecimalOutOfRange)
	}
}

func TestDecimalDecodeSources(t *testing.T) {
	t.Parallel()

	decode := func(v any) (money, error) {
		raw, err := bson.Marshal(bson.M{"amount": v})
		if err != nil {
			t.Fatal(err)
		}

		var out money

		return out, unmarshal(raw, &out)
	}

	out, err := decode(int32(7))
	if err != nil {
		t.Fatal(err)
	}

	if !decimal.NewFromInt(7).Equal(out.Amount) {
		t.Fatal("expected true: decimal.NewFromInt(7).Equal(out.Amount)")
	}

	out, err = decode(int64(1) << 40)
	if err != nil {
		t.Fatal(err)
	}

	if !decimal.NewFromInt(1 << 40).Equal(out.Amount) {
		t.Fatal("expected true: decimal.NewFromInt(1<<40).Equal(out.Amount)")
	}

	_, err = decode(1.5)
	if err == nil {
		t.Log("a double is not silently converted")
		t.Fatal("expected an error from err")
	}

	nan, err := bson.ParseDecimal128("NaN")
	if err != nil {
		t.Fatal(err)
	}

	_, err = decode(nan)
	if !errors.Is(err, bsoncodec.ErrDecimalNotFinite) {
		t.Fatalf("got %v, want %v", err, bsoncodec.ErrDecimalNotFinite)
	}

	inf, err := bson.ParseDecimal128("-Infinity")
	if err != nil {
		t.Fatal(err)
	}

	_, err = decode(inf)
	if !errors.Is(err, bsoncodec.ErrDecimalNotFinite) {
		t.Fatalf("got %v, want %v", err, bsoncodec.ErrDecimalNotFinite)
	}

	out, err = decode(nil)
	if err != nil {
		t.Fatal(err)
	}

	if !out.Amount.IsZero() {
		t.Log("null decodes to zero")
		t.Fatal("expected true: out.Amount.IsZero()")
	}
}

// TestDecimalCanonicalForm pins decimal128.Canonical, the stored form of a fitting decimal: the same value with a bounded
// coefficient and exponent, so a zero or a clamped exponent no longer carries the caller's extreme exponent.
func TestDecimalCanonicalForm(t *testing.T) {
	t.Parallel()

	for name, d := range map[string]decimal.Decimal{
		"zero, max exponent":       decimal.New(0, 2147483647),
		"zero, min exponent":       decimal.New(0, -2147483648),
		"plain":                    decimal.RequireFromString("-123.4500"),
		"clamped exponent":         decimal.New(1, 6144),
		"folded negative exponent": decimal.New(10, -6177),
		"trailing zeros":           decimal.RequireFromString(strings.Repeat("9", 34) + "0000"),
		"max":                      decimal.New(1, 6111),
		"min":                      decimal.New(1, -6176),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, ok := decimal128.Canonical(d)
			if !ok {
				t.Fatal("expected true: ok")
			}

			if !decimal128.Fits(got) {
				t.Fatal("expected true: decimal128.Fits(got)")
			}

			if got.Exponent() < int32(-6176) {
				t.Fatal("GreaterOrEqual assertion failed for got.Exponent()")
			}

			if got.Exponent() > int32(6111) {
				t.Fatal("LessOrEqual assertion failed for got.Exponent()")
			}

			if len(got.Coefficient().String()) > 35 {
				t.Fatal("LessOrEqual assertion failed for len(got.Coefficient().String())")
			} // 34 digits and a sign

			if d.Sign() == 0 {
				if !got.Equal(decimal.Zero) {
					t.Fatal("expected true: got.Equal(decimal.Zero)")
				}

				return
			}

			if got.Cmp(d) != 0 {
				t.Log("the value is kept")
				t.Fatal("expected zero: got.Cmp(d)")
			}
		})
	}

	for _, d := range []decimal.Decimal{decimal.New(1, 6145), decimal.New(1, -6177), decimal.RequireFromString(strings.Repeat("9", 35))} {
		_, ok := decimal128.Canonical(d)
		if ok {
			t.Fatal("expected false: ok")
		}
	}
}

// TestDecimalCanonicalMatchesDriver pins decimal128.Canonical and decimal128.Fits to the driver's own Decimal128 round trip: the
// canonical form is exactly the coefficient and exponent the driver stores for d and reads back, trailing zeros included.
func TestDecimalCanonicalMatchesDriver(t *testing.T) {
	t.Parallel()

	for _, d := range decimal128Corpus() {
		d128, fits := bson.ParseDecimal128FromBigInt(d.Coefficient(), int(d.Exponent()))
		if want, got := fits, decimal128.Fits(d); !reflect.DeepEqual(want, got) {
			t.Logf("%s e%d", d.Coefficient(), d.Exponent())
			t.Fatalf("got %#v, want %#v", got, want)
		}

		got, ok := decimal128.Canonical(d)
		if want, got := fits, ok; !reflect.DeepEqual(want, got) {
			t.Fatalf("got %#v, want %#v", got, want)
		}

		if !fits {
			continue
		}

		coefficient, exp, err := d128.BigInt()
		if err != nil {
			t.Fatal(err)
		}

		want := decimal.NewFromBigInt(coefficient, int32(exp)) //nolint:gosec // Decimal128 exponents fit in int32
		if want.IsZero() {
			want = decimal.Zero
		}

		if want, got := want.Coefficient().String(), got.Coefficient().String(); !reflect.DeepEqual(want, got) {
			t.Logf("%s e%d", d.Coefficient(), d.Exponent())
			t.Fatalf("got %#v, want %#v", got, want)
		}

		if want, got := want.Exponent(), got.Exponent(); !reflect.DeepEqual(want, got) {
			t.Logf("%s e%d", d.Coefficient(), d.Exponent())
			t.Fatalf("got %#v, want %#v", got, want)
		}
	}
}

// decimal128Corpus returns decimals around every Decimal128 boundary: coefficient lengths about 34 digits with and without trailing
// zeros, exponents about both ends of the range, zeros with any exponent, and a deterministic random spread.
func decimal128Corpus() []decimal.Decimal {
	coefficients := []string{"0", "1", "-1", "7", "10", "-1200", "123456789", strings.Repeat("9", 33), strings.Repeat("9", 34),
		"-" + strings.Repeat("9", 34), strings.Repeat("9", 35), strings.Repeat("9", 34) + "0", strings.Repeat("9", 34) + "000",
		"1" + strings.Repeat("0", 40), "-1" + strings.Repeat("0", 34), "12" + strings.Repeat("0", 33), "123" + strings.Repeat("0", 33)}
	exponents := []int32{0, 1, -1, 2, -2, 32, -33, 6111, 6110, 6112, 6144, 6145, 6146, -6176, -6175, -6177, -6178, -6200, -6210,
		6077, 6078, 6100, -2147483648, 2147483647}

	var out []decimal.Decimal

	for _, c := range coefficients {
		for _, e := range exponents {
			out = append(out, decimal.RequireFromString(c+"e"+strconv.Itoa(int(e))))
		}
	}

	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // deterministic test corpus

	for range 2000 {
		var sb strings.Builder
		if rng.IntN(2) == 0 {
			sb.WriteByte('-')
		}

		sb.WriteByte(byte('1' + rng.IntN(9)))

		for range rng.IntN(36) {
			sb.WriteByte(byte('0' + rng.IntN(10)))
		}

		sb.WriteString(strings.Repeat("0", rng.IntN(6)))
		e := int32(rng.IntN(2*6300) - 6300) //nolint:gosec // bounded
		out = append(out, decimal.RequireFromString(sb.String()+"e"+strconv.Itoa(int(e))))
	}

	return out
}
