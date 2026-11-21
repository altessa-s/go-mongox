// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package keyset

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-aip/pagination/pagetoken"

	"github.com/altessa-s/go-mongox/bsonfilter"
)

func TestHistoricalTokens(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	codec, err := pagetoken.New(bytes.Repeat([]byte{0x42}, 32), pagetoken.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}

	keys := []sortKey{{"rank", true}, {"identity.seq", false}}
	doc := bson.D{{Key: "meta", Value: bson.M{"b": int64(2), "a": int32(1)}}}

	canonical, err := bsonfilter.Prepare(t.Context(), doc, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	binding, err := newBindings(canonical, keys, "fixture-reader/precision=1ms")
	if err != nil {
		t.Fatal(err)
	}

	for _, order := range []string{"sorted", "reversed"} {
		data, err := os.ReadFile("../testdata/tokens-" + order + ".json")
		if err != nil {
			t.Fatal(err)
		}

		var fixture struct {
			Filter  string `json:"filter_hex"`
			Payload string `json:"payload_hex"`
			Token   string
			Sort    string `json:"sort_binding"`
			Subject string `json:"subject_binding"`
		}
		if err := json.Unmarshal(data, &fixture); err != nil {
			t.Fatal(err)
		}

		historical, err := hex.DecodeString(fixture.Filter)
		if err != nil {
			t.Fatal(err)
		}

		original, err := newBindings(historical, keys, binding.Subject)
		if err != nil {
			t.Fatal(err)
		}

		if original.Sort != fixture.Sort || original.Subject != fixture.Subject {
			t.Fatal("historical bindings changed")
		}

		payload, err := codec.Resolve(fixture.Token, original)
		if err != nil || hex.EncodeToString(payload) != fixture.Payload {
			t.Fatalf("resolve historical %s: %v", order, err)
		}

		values, err := decodePosition(payload, 2)
		if err != nil || values[0].Int32() != 7 || values[1].Int64() != 42 {
			t.Fatalf("position: %v", err)
		}

		issued, err := codec.Issue(payload, original)
		if err != nil || issued != fixture.Token {
			t.Fatalf("issue historical %s: %v", order, err)
		}

		_, err = codec.Resolve(fixture.Token, binding)
		if order == "sorted" && err != nil || order == "reversed" && !errors.Is(err, pagetoken.ErrFilterChanged) {
			t.Fatalf("resolve canonical %s: %v", order, err)
		}

		for _, tc := range []struct {
			binding pagetoken.Bindings
			want    error
		}{{pagetoken.Bindings{Sort: "other", Filter: original.Filter, Subject: original.Subject}, pagetoken.ErrSortChanged}, {pagetoken.Bindings{Sort: original.Sort, Filter: original.Filter, Subject: "other"}, pagetoken.ErrSubjectMismatch}} {
			if _, err := codec.Resolve(fixture.Token, tc.binding); !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		}

		now = now.Add(25 * time.Hour)
		_, err = codec.Resolve(fixture.Token, original)
		now = now.Add(-25 * time.Hour)

		if !errors.Is(err, pagetoken.ErrExpiredToken) {
			t.Fatalf("expiry: %v", err)
		}
	}
}
func FuzzDecodePosition(f *testing.F) {
	valid, err := bson.Marshal(bson.D{{Key: "k", Value: bson.A{int32(1), int64(2)}}})
	if err != nil {
		f.Fatal(err)
	}

	f.Add(valid, uint8(2))
	f.Add(append(bytes.Clone(valid), 0), uint8(2))
	f.Add([]byte{}, uint8(0))
	f.Fuzz(func(t *testing.T, payload []byte, n uint8) {
		values, err := decodePosition(payload, int(n))
		if err != nil {
			if !errors.Is(err, pagetoken.ErrInvalidToken) {
				t.Fatal(err)
			}

			return
		}

		if bson.Raw(payload).Validate() != nil {
			t.Fatal("accepted malformed BSON")
		}

		if len(values) != int(n) {
			t.Fatal("accepted wrong key count")
		}

		for _, v := range values {
			if err := v.Validate(); err != nil {
				t.Fatal(err)
			}
		}
	})
}
