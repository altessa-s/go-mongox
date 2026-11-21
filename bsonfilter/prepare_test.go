// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/altessa-s/go-mongox/bsonfilter"
)

func FuzzCanonicalMaps(f *testing.F) {
	f.Add(int64(7), "rank")
	f.Fuzz(func(t *testing.T, n int64, s string) {
		if len(s) > 1<<16 {
			t.Skip()
		}

		left := bson.M{"z": n, "a": s}
		right := map[string]any{"a": s, "z": n}
		ordered := bson.D{{Key: "z", Value: n}, {Key: "a", Value: s}}
		doc := func(v any) bson.D {
			return bson.D{{Key: "nested", Value: []any{v, v}}, {Key: "ordered", Value: ordered}}
		}

		want, err := bson.Marshal(doc(bson.D{{Key: "a", Value: s}, {Key: "z", Value: n}}))
		if err != nil {
			t.Fatal(err)
		}

		for _, m := range []any{left, right} {
			got, err := bsonfilter.Prepare(t.Context(), doc(m), nil, nil)
			if err != nil || !bytes.Equal(got, want) {
				t.Fatalf("unexpected canonical BSON: %v", err)
			}
		}

		if !reflect.DeepEqual(left, bson.M{"a": s, "z": n}) || !reflect.DeepEqual(right, map[string]any{"a": s, "z": n}) || ordered[0].Key != "z" {
			t.Fatal("input changed")
		}
	})
}
func TestPrepareDocuments(t *testing.T) {
	t.Parallel()

	for _, doc := range []any{nil, bson.D(nil), bson.M(nil), map[string]any(nil)} {
		got, err := bsonfilter.Prepare(t.Context(), doc, nil, nil)
		if err != nil || !bytes.Equal(got, []byte{5, 0, 0, 0, 0}) {
			t.Fatalf("%T: %v", doc, err)
		}
	}

	for _, doc := range []any{bson.E{}, bson.A{}, struct{}{}, []bson.E{}} {
		if _, err := bsonfilter.Prepare(t.Context(), doc, nil, nil); !errors.Is(err, bsonfilter.ErrValueType) {
			t.Fatalf("%T: %v", doc, err)
		}
	}

	for _, doc := range []any{bson.M{"z": int64(42), "a": 0}, map[string]any{"a": 0, "z": int64(42)}} {
		got, err := bsonfilter.Prepare(t.Context(), doc, nil, &options.BSONOptions{IntMinSize: true})
		if err != nil || got.Lookup("z").Type != bson.TypeInt32 {
			t.Fatalf("%T: %v", doc, err)
		}
	}

	raw, err := bson.Marshal(bson.D{{Key: "z", Value: 1}, {Key: "a", Value: 2}})
	if err != nil {
		t.Fatal(err)
	}

	got, err := bsonfilter.Prepare(t.Context(), bson.Raw(raw), nil, nil)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("raw changed")
	}

	raw[0] = 0

	if got[0] == 0 {
		t.Fatal("raw aliases input")
	}
}
func TestPrepareChecksCustomEncoder(t *testing.T) {
	t.Parallel()

	type scalar struct{}

	for _, operator := range []string{"$where", "$function", "$accumulator", ""} {
		reg := bson.NewRegistry()
		secret := errors.New("private filter value")

		reg.RegisterTypeEncoder(reflect.TypeFor[scalar](), bson.ValueEncoderFunc(func(_ bson.EncodeContext, vw bson.ValueWriter, _ reflect.Value) error {
			if operator == "" {
				return secret
			}

			dw, err := vw.WriteDocument()
			if err != nil {
				return err
			}

			element, err := dw.WriteDocumentElement(operator)
			if err != nil {
				return err
			}

			if err := element.WriteString(secret.Error()); err != nil {
				return err
			}

			return dw.WriteDocumentEnd()
		}))
		bsonfilter.AdmitScalar(reg, reflect.TypeFor[scalar]())
		_, err := bsonfilter.Prepare(t.Context(), bson.M{"field": scalar{}}, reg, nil)

		want := bsonfilter.ErrDangerousOperator
		if operator == "" {
			want = secret
		}

		if !errors.Is(err, want) || strings.Contains(err.Error(), secret.Error()) {
			t.Fatalf("operator %q: wrong or unredacted error", operator)
		}
	}
}

func TestRawParityCorpus(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../testdata/raw-filters.json")
	if err != nil {
		t.Fatal(err)
	}

	var cases []struct {
		Name, Error string
		BSON        string `json:"bson_hex"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}

	sentinels := map[string]error{"": nil, "ErrMalformedRaw": bsonfilter.ErrMalformedRaw, "ErrValueType": bsonfilter.ErrValueType, "ErrDangerousOperator": bsonfilter.ErrDangerousOperator}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			input, err := hex.DecodeString(tc.BSON)
			if err != nil {
				t.Fatal(err)
			}

			want, ok := sentinels[tc.Error]
			if !ok {
				t.Fatal("unknown fixture error", tc.Error)
			}

			got, err := bsonfilter.Prepare(t.Context(), bson.Raw(input), nil, nil)
			if !errors.Is(err, want) || err == nil && !bytes.Equal(got, input) {
				t.Fatalf("prepare: %v, want %v", err, want)
			}
		})
	}
}
