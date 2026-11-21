// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsoncodec_test

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-atlas-core/types/optional"
	"github.com/altessa-s/go-atlas-core/types/redacted"

	"github.com/altessa-s/go-mongox/bsoncodec"
)

// civilDate has the fields of cloud.google.com/go/civil.Date, which this module does not depend on: the tests register it with
// RegisterDateType, as a service registers that type.
type civilDate struct {
	Year  int
	Month time.Month
	Day   int
}

// IsZero reports whether d is the zero date.
func (d civilDate) IsZero() bool { return d == civilDate{} }

// testRegistry is NewRegistry with civilDate registered.
func testRegistry() *bson.Registry {
	reg := bsoncodec.NewRegistry()
	if err := bsoncodec.RegisterDateType[civilDate](reg); err != nil {
		panic(err)
	}

	return reg
}

func marshal(v any) ([]byte, error) {
	var out bytes.Buffer

	enc := bson.NewEncoder(bson.NewDocumentWriter(&out))
	enc.SetRegistry(testRegistry())
	err := enc.Encode(v)

	return out.Bytes(), err
}

func unmarshal(data []byte, v any) error {
	dec := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(data)))
	dec.SetRegistry(testRegistry())

	return dec.Decode(v)
}

func TestBSONRoundTrip_StructField(t *testing.T) {
	t.Parallel()

	type doc struct {
		DeletedAt optional.Optional[time.Time] `bson:"deleted_at,omitempty"`
		Note      optional.Optional[string]    `bson:"note,omitempty"`
	}

	t.Run("some_round_trips", func(t *testing.T) {
		t.Parallel()

		// Truncate to BSON's millisecond datetime precision.
		when := time.Now().UTC().Truncate(time.Millisecond)
		in := doc{
			DeletedAt: optional.Some(when),
			Note:      optional.Some(""),
		}

		raw, err := marshal(in)
		if err != nil {
			t.Fatal(err)
		}

		var out doc
		if err := unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}

		gotTime, ok := out.DeletedAt.Get()
		if !ok {
			t.Fatal("expected true: ok")
		}

		if !when.Equal(gotTime) {
			t.Logf("want %v got %v", when, gotTime)
			t.Fatal("expected true: when.Equal(gotTime)")
		}

		gotNote, ok := out.Note.Get()
		if !ok {
			t.Log("Some(\"\") must round-trip as Some, not None")
			t.Fatal("expected true: ok")
		}

		if want, got := "", gotNote; want != got {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("none_is_omitted_with_omitempty", func(t *testing.T) {
		t.Parallel()

		raw, err := marshal(doc{})
		if err != nil {
			t.Fatal(err)
		}

		var m bson.M
		if err := unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}

		if _, exists := m["deleted_at"]; exists {
			t.Fatal("NotContains assertion failed for m")
		}

		if _, exists := m["note"]; exists {
			t.Fatal("NotContains assertion failed for m")
		}
	})

	t.Run("missing_field_decodes_as_none", func(t *testing.T) {
		t.Parallel()

		raw, err := marshal(bson.M{})
		if err != nil {
			t.Fatal(err)
		}

		var out doc
		if err := unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}

		if !out.DeletedAt.IsNone() {
			t.Fatal("expected true: out.DeletedAt.IsNone()")
		}

		if !out.Note.IsNone() {
			t.Fatal("expected true: out.Note.IsNone()")
		}
	})

	t.Run("explicit_null_decodes_as_none", func(t *testing.T) {
		t.Parallel()

		raw, err := marshal(bson.M{"deleted_at": nil, "note": nil})
		if err != nil {
			t.Fatal(err)
		}

		var out doc
		if err := unmarshal(raw, &out); err != nil {
			t.Fatal(err)
		}

		if !out.DeletedAt.IsNone() {
			t.Fatal("expected true: out.DeletedAt.IsNone()")
		}

		if !out.Note.IsNone() {
			t.Fatal("expected true: out.Note.IsNone()")
		}
	})
}

func TestBSONRoundTrip_PointerInsideOptional(t *testing.T) {
	t.Parallel()

	type inner struct {
		Label string `bson:"label"`
	}

	type doc struct {
		Nested optional.Optional[*inner] `bson:"nested"`
	}

	in := doc{Nested: optional.Some(&inner{Label: "hi"})}

	raw, err := marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	var out doc
	if err := unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	got, ok := out.Nested.Get()
	if !ok {
		t.Fatal("expected true: ok")
	}

	if got == nil {
		t.Fatal("expected non-nil: got")
	}

	if want, got := "hi", got.Label; want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestUnconfiguredRegistryRejectsOptional(t *testing.T) {
	t.Parallel()

	_, err := bson.Marshal(bson.M{"value": optional.Some(42)})
	if !errors.Is(err, optional.ErrBSONCodecRequired) {
		t.Fatalf("got %v, want %v", err, optional.ErrBSONCodecRequired)
	}

	raw, err := bson.Marshal(bson.M{"value": 42})
	if err != nil {
		t.Fatal(err)
	}

	var out struct {
		Value optional.Optional[int] `bson:"value"`
	}
	if got := bson.Unmarshal(raw, &out); !errors.Is(got, optional.ErrBSONCodecRequired) {
		t.Fatalf("got %v, want %v", got, optional.ErrBSONCodecRequired)
	}
}

func TestCustomInnerCodecAndOtherMarshalers(t *testing.T) {
	t.Parallel()

	reg := bsoncodec.NewRegistry()
	reg.RegisterTypeEncoder(reflect.TypeFor[int](), bson.ValueEncoderFunc(func(_ bson.EncodeContext, w bson.ValueWriter, _ reflect.Value) error { return w.WriteString("custom") }))

	var out bytes.Buffer

	enc := bson.NewEncoder(bson.NewDocumentWriter(&out))
	enc.SetRegistry(reg)

	if err := enc.Encode(bson.M{"value": optional.Some(42), "secret": redacted.RedactedString("secret")}); err != nil {
		t.Fatal(err)
	}

	raw := bson.Raw(out.Bytes())
	if want, got := "custom", raw.Lookup("value").StringValue(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := "<redacted>", raw.Lookup("secret").StringValue(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	var decoded struct {
		Secret redacted.RedactedString `bson:"secret"`
	}
	if err := unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}

	if want, got := "<redacted>", decoded.Secret.Expose(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestDecodeErrorPreservesValue(t *testing.T) {
	t.Parallel()

	in := struct {
		Value optional.Optional[int] `bson:"value"`
	}{optional.Some(7)}

	raw, err := bson.Marshal(bson.M{"value": "invalid"})
	if err != nil {
		t.Fatal(err)
	}

	if unmarshal(raw, &in) == nil {
		t.Fatal("expected an error from unmarshal(raw, &in)")
	}

	if want, got := optional.Some(7), in.Value; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
