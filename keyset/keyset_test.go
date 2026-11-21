// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package keyset

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/altessa-s/go-aip/pagination/pagetoken"

	"github.com/altessa-s/go-mongox/bsonfilter"
)

func TestKeys(t *testing.T) {
	t.Parallel()

	keys, err := sortKeys(bson.D{{Key: "rank", Value: -1}, {Key: "name", Value: int32(1)}}, "")
	if err != nil {
		t.Fatal(err)
	}

	if want, got := []sortKey{{"rank", true}, {"name", false}, {"_id", false}}, keys; !reflect.DeepEqual(want, got) {
		t.Log("_id tie-breaker appended")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	keys, err = sortKeys(bson.D{{Key: "_id", Value: int64(-1)}, {Key: "ignored", Value: 1}}, "")
	if err != nil {
		t.Fatal(err)
	}

	if want, got := []sortKey{{"_id", true}}, keys; !reflect.DeepEqual(want, got) {
		t.Log("nothing breaks a tie after _id")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	keys, err = sortKeys(nil, "")
	if err != nil {
		t.Fatal(err)
	}

	if want, got := []sortKey{{"_id", false}}, keys; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	for name, sort := range map[string]bson.D{
		"direction_2":   {{Key: "a", Value: 2}},
		"text_score":    {{Key: "a", Value: bson.D{{Key: "$meta", Value: "textScore"}}}},
		"string_dir":    {{Key: "a", Value: "asc"}},
		"empty_field":   {{Key: "", Value: 1}},
		"operator":      {{Key: "$natural", Value: 1}},
		"repeated_name": {{Key: "a", Value: 1}, {Key: "a", Value: -1}},
	} {
		_, err := sortKeys(sort, "")
		if !errors.Is(err, ErrSort) {
			t.Log(name)
			t.Fatalf("got %v, want %v", err, ErrSort)
		}
	}
}

func TestPositionRoundTrip(t *testing.T) {
	t.Parallel()

	keys := []sortKey{{"meta.rank", true}, {"_id", false}}

	doc, err := bson.Marshal(bson.D{{Key: "_id", Value: "x1"}, {Key: "meta", Value: bson.D{{Key: "rank", Value: 7}}}})
	if err != nil {
		t.Fatal(err)
	}

	payload, err := encodePosition(doc, keys, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	values, err := decodePosition(payload, len(keys))
	if err != nil {
		t.Fatal(err)
	}

	if want, got := int32(7), values[0].Int32(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := "x1", values[1].StringValue(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	_, err = decodePosition(payload, 3)
	if !errors.Is(err, pagetoken.ErrInvalidToken) {
		t.Log("a key count mismatch is a bad token")
		t.Fatalf("got %v, want %v", err, pagetoken.ErrInvalidToken)
	}

	_, err = decodePosition([]byte("garbage"), 2)
	if !errors.Is(err, pagetoken.ErrInvalidToken) {
		t.Fatalf("got %v, want %v", err, pagetoken.ErrInvalidToken)
	}

	for name, d := range map[string]bson.D{
		"missing": {{Key: "_id", Value: "x1"}},
		"null":    {{Key: "_id", Value: "x1"}, {Key: "meta", Value: bson.D{{Key: "rank", Value: nil}}}},
		"array":   {{Key: "_id", Value: "x1"}, {Key: "meta", Value: bson.D{{Key: "rank", Value: bson.A{1}}}}},
	} {
		raw, err := bson.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}

		_, err = encodePosition(raw, keys, time.Millisecond)
		if !errors.Is(err, ErrMissingKey) {
			t.Log(name)
			t.Fatalf("got %v, want %v", err, ErrMissingKey)
		}
	}
}

func TestWithSeek(t *testing.T) {
	t.Parallel()

	v := func(x any) bson.RawValue {
		typ, data, err := bson.MarshalValue(x)
		if err != nil {
			t.Fatal(err)
		}

		return bson.RawValue{Type: typ, Value: data}
	}
	keys := []sortKey{{"rank", true}, {"_id", false}}
	got := withSeek(mustFilter(t, bson.D{{Key: "group", Value: "a"}}), keys, []bson.RawValue{v(3), v("id9")})

	want := bson.D{{Key: "$and", Value: bson.A{
		bson.D{{Key: "group", Value: "a"}},
		bson.D{{Key: "$or", Value: bson.A{
			bson.D{{Key: "$or", Value: bson.A{
				bson.D{{Key: "rank", Value: bson.D{{Key: "$lt", Value: v(3)}}}},
				bson.D{{Key: "rank", Value: nil}},
			}}},
			bson.D{
				{Key: "rank", Value: bson.D{{Key: "$eq", Value: v(3)}}},
				{Key: "_id", Value: bson.D{{Key: "$gt", Value: v("id9")}}},
			},
		}}},
	}}}

	gotJSON, err := bson.MarshalExtJSON(got, true, false)
	if err != nil {
		t.Fatal(err)
	}

	wantJSON, err := bson.MarshalExtJSON(want, true, false)
	if err != nil {
		t.Fatal(err)
	}

	if string(wantJSON) != string(gotJSON) {
		t.Fatal("JSONEq assertion failed for string(wantJSON)")
	}

	noFilter := withSeek(mustFilter(t, nil), keys[1:], []bson.RawValue{v("id9")})
	if want, got := "$or", noFilter[0].Key; want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if len(noFilter) != 1 {
		t.Log("an empty filter adds no $and")
		t.Fatal("Len assertion failed for noFilter")
	}
}

func mustFilter(tb testing.TB, filter bson.D) bson.Raw {
	tb.Helper()

	raw, err := bsonfilter.Prepare(tb.Context(), filter, nil, nil)
	if err != nil {
		tb.Fatal(err)
	}

	return raw
}

func mustBindings(tb testing.TB, filter bson.D, keys []sortKey, subject string) pagetoken.Bindings {
	tb.Helper()

	b := newBindings(mustFilter(tb, filter), keys, subject)

	return b
}

func TestBindings(t *testing.T) {
	t.Parallel()

	keys, err := sortKeys(bson.D{{Key: "rank", Value: -1}}, "")
	if err != nil {
		t.Fatal(err)
	}

	a := mustBindings(t, bson.D{{Key: "g", Value: "a"}}, keys, "s")

	b := mustBindings(t, bson.D{{Key: "g", Value: "b"}}, keys, "s")
	if want, got := a.Filter, b.Filter; reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := a.Sort, b.Sort; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := "s", a.Subject; want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	none := mustBindings(t, nil, keys, "")

	empty := mustBindings(t, bson.D{}, keys, "")
	if want, got := none.Filter, empty.Filter; !reflect.DeepEqual(want, got) {
		t.Log("nil and empty filters match the same documents")
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestFilterBindsTheExecutedBytes(t *testing.T) {
	t.Parallel()

	keys, err := sortKeys(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	// An embedded-document literal is compared by MongoDB in its key order: the executed filter and the binding keep it.
	filter := bson.D{{Key: "meta", Value: bson.D{{Key: "b", Value: 1}, {Key: "a", Value: 2}}}}
	raw := mustFilter(t, filter)

	want, err := bson.Marshal(filter)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := bson.Raw(want), raw; !reflect.DeepEqual(want, got) {
		t.Log("the filter is executed as written")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	elems, err := raw.Lookup("meta").Document().Elements()
	if err != nil {
		t.Fatal(err)
	}

	if want, got := "b", elems[0].Key(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := "a", elems[1].Key(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	bound := mustBindings(t, filter, keys, "")

	fingerprint, err := bson.Marshal(bson.D{{Key: "f", Value: filter}})
	if err != nil {
		t.Fatal(err)
	}

	if want, got := string(fingerprint), bound.Filter; !reflect.DeepEqual(want, got) {
		t.Log("the binding is the executed bytes")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	reordered := mustBindings(t, bson.D{{Key: "meta", Value: bson.D{{Key: "a", Value: 2}, {Key: "b", Value: 1}}}}, keys, "")
	if want, got := bound.Filter, reordered.Filter; reflect.DeepEqual(want, got) {
		t.Log("a reordered literal is another filter")
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestFindPageRejectsTokenWithoutCodec(t *testing.T) {
	t.Parallel()

	codec, err := pagetoken.New(bytes.Repeat([]byte{1}, pagetoken.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}

	token, err := codec.Issue([]byte("x"), pagetoken.Bindings{})
	if err != nil {
		t.Fatal(err)
	}

	_, err = FindPage[bson.M](t.Context(), nil, nil, Query{After: token})
	if !errors.Is(err, ErrTokensRequired) {
		t.Fatalf("got %v, want %v", err, ErrTokensRequired)
	}

	if !errors.Is(err, pagetoken.ErrCodecRequired) {
		t.Fatalf("got %v, want %v", err, pagetoken.ErrCodecRequired)
	}

	if want, got := pagetoken.FailureMalformed, pagetoken.Classify(err); !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestBindingsKeepTypes(t *testing.T) {
	t.Parallel()

	keys, err := sortKeys(nil, "")
	if err != nil {
		t.Fatal(err)
	}

	a := mustBindings(t, bson.D{{Key: "rank", Value: bson.D{{Key: "$eq", Value: int32(1)}}}}, keys, "")

	b := mustBindings(t, bson.D{{Key: "rank", Value: bson.D{{Key: "$eq",
		Value: bson.D{{Key: "$numberInt", Value: "1"}}}}}}, keys, "")
	if want, got := a.Filter, b.Filter; reflect.DeepEqual(want, got) {
		t.Log("an int32 and a $numberInt document are different filters")
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestCheckProjection(t *testing.T) {
	t.Parallel()

	keys := []sortKey{{"meta.rank", false}, {"_id", false}}
	for _, p := range []bson.D{
		nil,
		{{Key: "meta.rank", Value: 1}, {Key: "name", Value: bson.D{{Key: "$literal", Value: 0}}}},
		{{Key: "meta", Value: true}},
		{{Key: "secret", Value: 0}},
		{{Key: "meta.rank", Value: int64(1)}, {Key: "_id", Value: 1.0}},
		{{Key: "meta.rank.x", Value: false}},
		{{Key: "tags", Value: bson.D{{Key: "$slice", Value: 2}}}},
		{{Key: "tags", Value: bson.M{"$slice": 2}}, {Key: "secret", Value: false}},
	} {
		if err := checkProjection(p, keys); err != nil {
			t.Logf("%v", p)
			t.Fatal(err)
		}
	}

	for _, p := range []bson.D{
		{{Key: "meta.rank", Value: bson.D{{Key: "$literal", Value: 0}}}},
		{{Key: "meta", Value: "$other"}},
		{{Key: "meta.rank.x", Value: bson.D{{Key: "$slice", Value: 1}}}},
		{{Key: "meta.rank.$", Value: 1}},
		{{Key: "_id", Value: 2}},
		{{Key: "meta.rank", Value: int64(1)}, {Key: "_id", Value: 0.0}}, // drops the tie-breaker
		{{Key: "meta.rank", Value: 0}},
		{{Key: "meta", Value: false}},
		{{Key: "_id", Value: int32(0)}},
		{{Key: "name", Value: 1}},        // an inclusion without meta.rank
		{{Key: "meta.rank.x", Value: 1}}, // a descendant does not return the scalar
		{{Key: "_id", Value: true}},      // only _id
		{{Key: "tags", Value: bson.D{{Key: "$elemMatch", Value: bson.D{{Key: "a", Value: 1}}}}}},
		{{Key: "name", Value: bson.D{{Key: "$toUpper", Value: "$name"}}}},
	} {
		if got := checkProjection(p, keys); !errors.Is(got, ErrProjection) {
			t.Logf("%v", p)
			t.Fatalf("got %v, want %v", got, ErrProjection)
		}
	}
}

// celsius is a filter value only a custom registry can encode.
type celsius struct{ deg int32 }

func TestFilterUsesTheQueryRegistryAndOptions(t *testing.T) {
	t.Parallel()

	reg := bson.NewRegistry()
	reg.RegisterTypeEncoder(reflect.TypeFor[celsius](), bson.ValueEncoderFunc(
		func(_ bson.EncodeContext, vw bson.ValueWriter, v reflect.Value) error {
			return vw.WriteInt32(v.Interface().(celsius).deg) //nolint:forcetypeassert // registered for celsius only
		}))
	bsonfilter.AdmitScalar(reg, reflect.TypeFor[celsius]())

	raw, err := bsonfilter.Prepare(t.Context(), bson.D{{Key: "temp", Value: celsius{deg: 21}}}, reg, nil)
	if err != nil {
		t.Fatal(err)
	}

	if want, got := int32(21), raw.Lookup("temp").Int32(); want != got {
		t.Log("the custom codec of the query registry encodes the filter")
		t.Fatalf("got %#v, want %#v", got, want)
	}

	type bounds struct {
		Min int64 `bson:"min"`
	}

	_, err = bsonfilter.Prepare(t.Context(), bson.D{{Key: "range", Value: bounds{Min: 3}}}, nil, &options.BSONOptions{IntMinSize: true})
	if !errors.Is(err, bsonfilter.ErrValueType) {
		t.Fatalf("got %v, want %v", err, bsonfilter.ErrValueType)
	}
}

func TestConfiguredIdentityStopsSort(t *testing.T) {
	t.Parallel()

	keys, err := sortKeys(bson.D{{Key: "rank", Value: -1}, {Key: "identity.uuid", Value: -1}}, "identity.uuid")
	if err != nil {
		t.Fatal(err)
	}

	if want, got := []sortKey{{field: "rank", desc: true}, {field: "identity.uuid", desc: true}}, keys; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	keys, err = sortKeys(nil, "identity.uuid")
	if err != nil {
		t.Fatal(err)
	}

	if want, got := []sortKey{{field: "identity.uuid"}}, keys; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	_, err = sortKeys(nil, "$bad")
	if !errors.Is(err, ErrSort) {
		t.Fatalf("got %v, want %v", err, ErrSort)
	}
}

func TestCustomIdentityCursorWithoutMongoID(t *testing.T) {
	t.Parallel()

	keys, err := sortKeys(nil, "identity.sequence")
	if err != nil {
		t.Fatal(err)
	}

	document, err := bson.Marshal(bson.D{{Key: "identity", Value: bson.D{{Key: "sequence", Value: int64(42)}}}})
	if err != nil {
		t.Fatal(err)
	}

	payload, err := encodePosition(document, keys, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	values, err := decodePosition(payload, len(keys))
	if err != nil {
		t.Fatal(err)
	}

	if want, got := int64(42), values[0].Int64(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	seek := withSeek(mustFilter(t, nil), keys, values)
	if want, got := (bson.D{{Key: "$or", Value: bson.A{bson.D{{Key: "identity.sequence", Value: bson.D{{Key: "$gt", Value: values[0]}}}}}}}), seek; !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if got := checkProjection(bson.D{{Key: "identity", Value: bson.D{{Key: "$literal", Value: 0}}}}, keys); !errors.Is(got, ErrProjection) {
		t.Fatalf("got %v, want %v", got, ErrProjection)
	}
}

func TestCursorTimestampPrecision(t *testing.T) {
	t.Parallel()

	keys, err := sortKeys(bson.D{{Key: "at", Value: 1}}, "sequence")
	if err != nil {
		t.Fatal(err)
	}

	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	document, err := bson.Marshal(bson.D{{Key: "at", Value: at}, {Key: "sequence", Value: int64(42)}})
	if err != nil {
		t.Fatal(err)
	}

	payload, err := encodePosition(document, keys, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	values, err := decodePosition(payload, len(keys))
	if err != nil {
		t.Fatal(err)
	}

	if want, got := at.UnixMilli(), values[0].DateTime(); !reflect.DeepEqual(want, got) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := int64(42), values[1].Int64(); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	document, err = bson.Marshal(bson.D{{Key: "at", Value: at.Add(time.Millisecond)}, {Key: "sequence", Value: int64(42)}})
	if err != nil {
		t.Fatal(err)
	}

	_, err = encodePosition(document, keys, time.Second)
	if !errors.Is(err, ErrMissingKey) {
		t.Fatalf("got %v, want %v", err, ErrMissingKey)
	}
}

// TestFindPageRefusesBeforeQuerying checks every failure FindPage decides before it sends a command (the collection is nil).
func TestFindPageRefusesBeforeQuerying(t *testing.T) {
	t.Parallel()

	codec, err := pagetoken.New(bytes.Repeat([]byte{1}, pagetoken.MinKeyLength))
	if err != nil {
		t.Fatal(err)
	}

	keys, err := sortKeys(nil, "")
	if err != nil {
		t.Fatal(err)
	}

	otherBindings := newBindings(mustFilter(t, bson.D{{Key: "g", Value: "other"}}), keys, "s/precision=1ms")

	foreign, err := codec.Issue([]byte("x"), otherBindings)
	if err != nil {
		t.Fatal(err)
	}

	bindings := newBindings(mustFilter(t, nil), keys, "s/precision=1ms")

	garbage, err := codec.Issue([]byte("not a position"), bindings)
	if err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		q    Query
		want error
	}{
		"dangerous filter":   {q: Query{Filter: bson.D{{Key: "$where", Value: "1"}}}, want: bsonfilter.ErrDangerousOperator},
		"precision":          {q: Query{TimestampPrecision: time.Microsecond}, want: ErrPrecision},
		"negative precision": {q: Query{TimestampPrecision: -time.Second}, want: ErrPrecision},
		"sort":               {q: Query{Sort: bson.D{{Key: "a", Value: 2}}}, want: ErrSort},
		"sort NUL":           {q: Query{Sort: bson.D{{Key: "a\x00b", Value: 1}}}, want: ErrSort},
		"projection":         {q: Query{Projection: bson.D{{Key: "_id", Value: "$x"}}}, want: ErrProjection},
		"dropped sort field": {q: Query{Sort: bson.D{{Key: "a", Value: 1}}, Projection: bson.D{{Key: "b", Value: 1}}}, want: ErrProjection},
		"unencodable filter": {q: Query{Filter: bson.D{{Key: "f", Value: func() {}}}}, want: bsonfilter.ErrValueType},
		"token of a filter":  {q: Query{After: foreign, Subject: "s"}, want: pagetoken.ErrFilterChanged},
		"malformed position": {q: Query{After: garbage, Subject: "s"}, want: pagetoken.ErrInvalidToken},
	} {
		_, err := FindPage[bson.M](t.Context(), nil, codec, tc.q)
		if err == nil {
			t.Log(name)
			t.Fatal("expected an error from err")
		}

		if tc.want != nil {
			if !errors.Is(err, tc.want) {
				t.Log(name)
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		}
	}

	if want, got := int32(1), sortDirection(false); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	if want, got := int32(-1), sortDirection(true); want != got {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
