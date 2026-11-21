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

	"github.com/altessa-s/go-mongox/bsoncodec"

	atlascivil "github.com/altessa-s/go-atlas-core/time/civil"
)

type period struct {
	From civilDate  `bson:"from"`
	To   *civilDate `bson:"to"`
}

func TestCivilDateRoundTrip(t *testing.T) {
	t.Parallel()

	to := civilDate{Year: 2026, Month: time.December, Day: 31}
	in := period{From: civilDate{Year: 2026, Month: time.February, Day: 28}, To: &to}

	raw, err := marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := "2026-02-28", bson.Raw(raw).Lookup("from").StringValue(); !reflect.DeepEqual(want, got) {
		t.Log("stored as YYYY-MM-DD")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := "2026-12-31", bson.Raw(raw).Lookup("to").StringValue(); !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	var out period
	if err := unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if want, got := in.From, out.From; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := to, *out.To; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestCivilDateZeroAndNil(t *testing.T) {
	t.Parallel()

	raw, err := marshal(period{})
	if err != nil {
		t.Fatal(err)
	}

	if want, got := bson.TypeNull, bson.Raw(raw).Lookup("from").Type; !reflect.DeepEqual(want, got) {
		t.Log("the zero date is null")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := bson.TypeNull, bson.Raw(raw).Lookup("to").Type; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	var out period
	if err := unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if !out.From.IsZero() {
		t.Fatal("expected true: out.From.IsZero()")
	}

	if out.To != nil {
		t.Fatal("expected nil: out.To")
	}
}

func TestCivilDateSortsAsStored(t *testing.T) {
	t.Parallel()

	a, err := marshal(period{From: civilDate{Year: 999, Month: time.January, Day: 1}})
	if err != nil {
		t.Fatal(err)
	}

	b, err := marshal(period{From: civilDate{Year: 2026, Month: time.January, Day: 1}})
	if err != nil {
		t.Fatal(err)
	}

	if bson.Raw(a).Lookup("from").StringValue() >= bson.Raw(b).Lookup("from").StringValue() {
		t.Log("years are zero-padded")
		t.Fatal("Less assertion failed for bson.Raw(a).Lookup(\"from\").StringValue()")
	}
}

func TestCivilDateInvalid(t *testing.T) {
	t.Parallel()

	for _, d := range []civilDate{
		{Year: 2026, Month: time.February, Day: 30},
		{Year: 10000, Month: time.January, Day: 1},
		{Year: -1, Month: time.January, Day: 1},
	} {
		_, err := marshal(period{From: d})
		if !errors.Is(err, bsoncodec.ErrInvalidDate) {
			t.Logf("%v", d)
			t.Fatalf("got %v, want %v", err, bsoncodec.ErrInvalidDate)
		}
	}

	for _, s := range []string{"2026-02-30", "2026-2-1", "26-02-01", "2026-02-01T00:00:00Z", " "} {
		raw, err := bson.Marshal(bson.M{"from": s})
		if err != nil {
			t.Fatal(err)
		}

		var out period
		if got := unmarshal(raw, &out); !errors.Is(got, bsoncodec.ErrInvalidDate) {
			t.Log(s)
			t.Fatalf("got %v, want %v", got, bsoncodec.ErrInvalidDate)
		}
	}

	raw, err := bson.Marshal(bson.M{"from": time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}

	var out period
	if unmarshal(raw, &out) == nil {
		t.Log("a BSON date is an instant, not a calendar date")
		t.Fatal("expected an error from unmarshal(raw, &out)")
	}
}

func TestCivilDateEmptyStringDecodesAsZero(t *testing.T) {
	t.Parallel()

	raw, err := bson.Marshal(bson.M{"from": ""})
	if err != nil {
		t.Fatal(err)
	}

	var out period
	if err := unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if !out.From.IsZero() {
		t.Fatal("expected true: out.From.IsZero()")
	}
}

// serviceDate has the shape of a service's own calendar date type, with a
// method set of its own.
type serviceDate struct {
	Year  int
	Month time.Month
	Day   int
}

func (d serviceDate) IsZero() bool { return d == serviceDate{} }

type invoice struct {
	Due  serviceDate  `bson:"due"`
	Paid *serviceDate `bson:"paid"`
}

func serviceRegistry(t *testing.T, opts ...bsoncodec.DateOption) *bson.Registry {
	t.Helper()

	reg := bsoncodec.NewRegistry()
	if err := bsoncodec.RegisterDateType[serviceDate](reg, opts...); err != nil {
		t.Fatal(err)
	}

	return reg
}

func marshalReg(t *testing.T, reg *bson.Registry, v any) bson.Raw {
	t.Helper()

	var buf bytes.Buffer

	enc := bson.NewEncoder(bson.NewDocumentWriter(&buf))
	enc.SetRegistry(reg)

	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}

	return buf.Bytes()
}

func unmarshalReg(t *testing.T, reg *bson.Registry, raw bson.Raw, v any) error {
	t.Helper()

	dec := bson.NewDecoder(bson.NewDocumentReader(bytes.NewReader(raw)))
	dec.SetRegistry(reg)

	return dec.Decode(v)
}

func TestRegisterDateTypeOwnType(t *testing.T) {
	t.Parallel()

	reg := serviceRegistry(t)
	paid := serviceDate{Year: 2026, Month: time.March, Day: 1}

	raw := marshalReg(t, reg, invoice{Due: serviceDate{Year: 2026, Month: time.February, Day: 28}, Paid: &paid})
	if want, got := "2026-02-28", raw.Lookup("due").StringValue(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := "2026-03-01", raw.Lookup("paid").StringValue(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	var out invoice
	if err := unmarshalReg(t, reg, raw, &out); err != nil {
		t.Fatal(err)
	}

	if want, got := (serviceDate{Year: 2026, Month: time.February, Day: 28}), out.Due; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := paid, *out.Paid; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	raw = marshalReg(t, reg, invoice{})
	if want, got := bson.TypeNull, raw.Lookup("due").Type; want != got {
		t.Log("the zero date is null by default")
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestRegisterDateTypeZeroAsEmptyString(t *testing.T) {
	t.Parallel()

	reg := serviceRegistry(t, bsoncodec.WithDateZeroAsEmptyString())

	raw := marshalReg(t, reg, invoice{})
	if want, got := bson.TypeString, raw.Lookup("due").Type; want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if len(raw.Lookup("due").StringValue()) != 0 {
		t.Log("the zero date is the empty string")
		t.Fatal("Empty assertion failed for raw.Lookup(\"due\").StringValue()")
	}

	if want, got := bson.TypeNull, raw.Lookup("paid").Type; want != got {
		t.Log("a nil pointer stays null")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	var out invoice
	if err := unmarshalReg(t, reg, raw, &out); err != nil {
		t.Fatal(err)
	}

	if !out.Due.IsZero() {
		t.Fatal("expected true: out.Due.IsZero()")
	}

	for _, bad := range []serviceDate{{Year: 2026, Month: 13, Day: 1}, {Year: 2026, Month: time.April, Day: 31}, {Year: 10000, Month: 1, Day: 1}} {
		var buf bytes.Buffer

		enc := bson.NewEncoder(bson.NewDocumentWriter(&buf))
		enc.SetRegistry(reg)

		if got := enc.Encode(invoice{Due: bad}); !errors.Is(got, bsoncodec.ErrInvalidDate) {
			t.Logf("%v", bad)
			t.Fatalf("got %v, want %v", got, bsoncodec.ErrInvalidDate)
		}
	}
}

func TestRegisterDateTypeRejectsOtherTypes(t *testing.T) {
	t.Parallel()

	type noDay struct {
		Year  int
		Month time.Month
	}

	type wrongMonth struct {
		Year, Month, Day int
	}

	type unexported struct {
		Year  int
		Month time.Month
		day   int
	}

	reg := bson.NewRegistry()
	if got := bsoncodec.RegisterDateType[noDay](reg); !errors.Is(got, bsoncodec.ErrNotADateType) {
		t.Fatalf("got %v, want %v", got, bsoncodec.ErrNotADateType)
	}

	if got := bsoncodec.RegisterDateType[wrongMonth](reg); !errors.Is(got, bsoncodec.ErrNotADateType) {
		t.Fatalf("got %v, want %v", got, bsoncodec.ErrNotADateType)
	}

	if got := bsoncodec.RegisterDateType[unexported](reg); !errors.Is(got, bsoncodec.ErrNotADateType) {
		t.Fatalf("got %v, want %v", got, bsoncodec.ErrNotADateType)
	}

	if got := bsoncodec.RegisterDateType[string](reg); !errors.Is(got, bsoncodec.ErrNotADateType) {
		t.Fatalf("got %v, want %v", got, bsoncodec.ErrNotADateType)
	}

	_ = unexported{}.day
}

type atlasPeriod struct {
	From atlascivil.Date  `bson:"from"`
	End  atlascivil.Date  `bson:"end"`
	To   *atlascivil.Date `bson:"to"`
}

func TestAtlasCivilDateRegisteredByDefault(t *testing.T) {
	t.Parallel()

	in := atlasPeriod{From: atlascivil.MustNew(2026, time.February, 28)}

	raw, err := marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := "2026-02-28", bson.Raw(raw).Lookup("from").StringValue(); !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := bson.TypeString, bson.Raw(raw).Lookup("end").Type; !reflect.DeepEqual(want, got) {
		t.Log("the zero date is stored as its text form")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if len(bson.Raw(raw).Lookup("end").StringValue()) != 0 {
		t.Fatal("Empty assertion failed for bson.Raw(raw).Lookup(\"end\").StringValue()")
	}

	if want, got := bson.TypeNull, bson.Raw(raw).Lookup("to").Type; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	var out atlasPeriod
	if err := unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}

	if want, got := in, out; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

// TestDateTypeValidRule pins that a date type's own Valid() rule bounds the
// codec: the go-atlas-core civil.Date excludes year 0, a civilDate without a Valid rule does not.
func TestDateTypeValidRule(t *testing.T) {
	t.Parallel()

	_, err := marshal(atlasPeriod{From: atlascivil.Date{Year: 0, Month: time.January, Day: 1}})
	if !errors.Is(err, bsoncodec.ErrInvalidDate) {
		t.Log("year 0 is not a go-atlas-core civil date")
		t.Fatalf("got %v, want %v", err, bsoncodec.ErrInvalidDate)
	}

	raw, err := bson.Marshal(bson.M{"from": "0000-01-01"})
	if err != nil {
		t.Fatal(err)
	}

	var out atlasPeriod
	if got := unmarshal(raw, &out); !errors.Is(got, bsoncodec.ErrInvalidDate) {
		t.Fatalf("got %v, want %v", got, bsoncodec.ErrInvalidDate)
	}

	var google period
	if err := unmarshal(raw, &google); err != nil {
		t.Log("a civilDate without a Valid rule accepts year 0")
		t.Fatal(err)
	}

	if want, got := 0, google.From.Year; want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

// pointerValidDate has its validity rule on the pointer receiver.
type pointerValidDate struct {
	Year  int
	Month time.Month
	Day   int
}

func (d *pointerValidDate) Valid() bool { return d.Year >= 1 }

func TestDateTypeValidRulePointerReceiver(t *testing.T) {
	t.Parallel()

	reg := bsoncodec.NewRegistry()
	if err := bsoncodec.RegisterDateType[pointerValidDate](reg); err != nil {
		t.Fatal(err)
	}

	type doc struct {
		D pointerValidDate `bson:"d"`
	}

	var buf bytes.Buffer

	enc := bson.NewEncoder(bson.NewDocumentWriter(&buf))
	enc.SetRegistry(reg)

	if got := enc.Encode(doc{D: pointerValidDate{Year: 0, Month: time.January, Day: 1}}); !errors.Is(got, bsoncodec.ErrInvalidDate) {
		t.Fatalf("got %v, want %v", got, bsoncodec.ErrInvalidDate)
	}

	raw, err := bson.Marshal(bson.M{"d": "0000-01-01"})
	if err != nil {
		t.Fatal(err)
	}

	var out doc
	if got := unmarshalReg(t, reg, raw, &out); !errors.Is(got, bsoncodec.ErrInvalidDate) {
		t.Fatalf("got %v, want %v", got, bsoncodec.ErrInvalidDate)
	}

	ok, err := bson.Marshal(bson.M{"d": "2026-10-09"})
	if err != nil {
		t.Fatal(err)
	}

	if err := unmarshalReg(t, reg, ok, &out); err != nil {
		t.Fatal(err)
	}

	if want, got := 2026, out.D.Year; want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
