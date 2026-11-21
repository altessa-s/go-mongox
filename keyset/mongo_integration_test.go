//go:build integration

// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package keyset_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/event"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/altessa-s/go-aip/pagination/pagetoken"

	"github.com/altessa-s/go-mongox/bsoncodec"
	"github.com/altessa-s/go-mongox/bsonfilter"
	"github.com/altessa-s/go-mongox/indexes"
	"github.com/altessa-s/go-mongox/keyset"
)

func collection(t *testing.T, opts *options.ClientOptions) *mongo.Collection {
	t.Helper()

	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("set MONGO_URI to run MongoDB integration tests")
	}

	client, err := mongo.Connect(opts.ApplyURI(uri).SetServerSelectionTimeout(10 * time.Second))
	if err != nil {
		t.Fatal(err)
	}

	coll := client.Database("go_mongox_it").Collection(bson.NewObjectID().Hex())

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 10*time.Second)
		defer cancel()

		if err := coll.Drop(ctx); err != nil {
			t.Error(err)
		}

		if err := client.Disconnect(ctx); err != nil {
			t.Error(err)
		}
	})

	return coll
}

type failingDocument struct{}

func (*failingDocument) UnmarshalBSON([]byte) error { return pagetoken.ErrInvalidToken }

func TestMongoPageTraversal(t *testing.T) {
	t.Parallel()

	reg := bsoncodec.NewRegistry()
	commands := make(chan bson.Raw, 16)
	monitor := event.CommandMonitor{Started: func(_ context.Context, event *event.CommandStartedEvent) {
		if event.CommandName == "find" {
			commands <- bson.Raw(bytes.Clone(event.Command.Lookup("filter").Document()))
		}
	}}
	coll := collection(t, options.Client().SetRegistry(reg).SetMonitor(&monitor))

	codec, err := pagetoken.New(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}

	for seq := range 9 {
		doc := bson.D{{Key: "rank", Value: seq % 3}, {Key: "identity", Value: bson.D{{Key: "seq", Value: int64(seq)}}}, {Key: "price", Value: decimal.NewFromInt(int64(seq))}}
		if _, err := coll.InsertOne(t.Context(), doc); err != nil {
			t.Fatal(err)
		}
	}

	q := keyset.Query{Filter: bson.D{{Key: "price", Value: bson.M{"$gte": decimal.NewFromInt(2), "$lt": decimal.NewFromInt(8)}}}, Registry: reg, Sort: bson.D{{Key: "rank", Value: -1}}, TiebreakPath: "identity.seq", Limit: 2, Subject: "reader"}

	want, err := bsonfilter.Prepare(t.Context(), q.Filter, reg, nil)
	if err != nil {
		t.Fatal(err)
	}

	var ids []int64

	for n := range 4 {
		page, err := keyset.FindPage[bson.Raw](t.Context(), coll, codec, q)
		if err != nil {
			t.Fatal(err)
		}

		sent := <-commands
		if n > 0 {
			sent = sent.Lookup("$and", "0").Document()
		}

		if !bytes.Equal(sent, want) {
			t.Fatalf("find filter %x, want %x", []byte(sent), []byte(want))
		}

		for _, row := range page.Items {
			ids = append(ids, row.Lookup("identity", "seq").Int64())
		}

		if page.Next == "" {
			break
		}

		q.After = page.Next
	}

	if !slices.Equal(ids, []int64{2, 5, 4, 7, 3, 6}) {
		t.Fatalf("page traversal: %v", ids)
	}

	q.After = ""

	q.Projection = bson.D{{Key: "rank", Value: 0}}
	if _, err := keyset.FindPage[bson.Raw](t.Context(), coll, codec, q); !errors.Is(err, keyset.ErrProjection) {
		t.Fatalf("projection: %v", err)
	}

	q.Projection = nil
	_, err = keyset.FindPage[failingDocument](t.Context(), coll, codec, q)

	var decode *keyset.DecodeError
	if !errors.As(err, &decode) || !errors.Is(err, pagetoken.ErrInvalidToken) {
		t.Fatalf("decoder failure: %v", err)
	}
}

// TestMongoDescendingNulls checks that a descending sort reaches the documents whose sort field is null or missing, which sort
// last: returned when they end the read, refused with ErrMissingKey when a page ends on one, never skipped.
func TestMongoDescendingNulls(t *testing.T) {
	t.Parallel()

	coll := collection(t, options.Client())

	codec, err := pagetoken.New(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}

	docs := []bson.D{
		{{Key: "g", Value: "tail"}, {Key: "_id", Value: int32(1)}, {Key: "rank", Value: int32(3)}},
		{{Key: "g", Value: "tail"}, {Key: "_id", Value: int32(2)}, {Key: "rank", Value: int32(2)}},
		{{Key: "g", Value: "tail"}, {Key: "_id", Value: int32(3)}, {Key: "rank", Value: int32(1)}},
		{{Key: "g", Value: "tail"}, {Key: "_id", Value: int32(4)}},
		{{Key: "g", Value: "many"}, {Key: "_id", Value: int32(5)}, {Key: "rank", Value: int32(1)}},
		{{Key: "g", Value: "many"}, {Key: "_id", Value: int32(6)}, {Key: "rank", Value: nil}},
		{{Key: "g", Value: "many"}, {Key: "_id", Value: int32(7)}},
		{{Key: "g", Value: "many"}, {Key: "_id", Value: int32(8)}},
	}
	for _, doc := range docs {
		if _, err := coll.InsertOne(t.Context(), doc); err != nil {
			t.Fatal(err)
		}
	}

	q := keyset.Query{Filter: bson.D{{Key: "g", Value: "tail"}}, Sort: bson.D{{Key: "rank", Value: -1}}, Limit: 2}

	var ids []int32

	for range 3 {
		page, err := keyset.FindPage[bson.Raw](t.Context(), coll, codec, q)
		if err != nil {
			t.Fatal(err)
		}

		for _, row := range page.Items {
			ids = append(ids, row.Lookup("_id").Int32())
		}

		if page.Next == "" {
			break
		}

		q.After = page.Next
	}

	if !slices.Equal(ids, []int32{1, 2, 3, 4}) {
		t.Fatalf("the document without rank must be read last: %v", ids)
	}

	q = keyset.Query{Filter: bson.D{{Key: "g", Value: "many"}}, Sort: bson.D{{Key: "rank", Value: -1}}, Limit: 1}

	page, err := keyset.FindPage[bson.Raw](t.Context(), coll, codec, q)
	if err != nil {
		t.Fatal(err)
	}

	q.After = page.Next
	if _, err := keyset.FindPage[bson.Raw](t.Context(), coll, codec, q); !errors.Is(err, keyset.ErrMissingKey) {
		t.Fatalf("a page ending on a null rank: %v", err)
	}
}

func TestMongoIndexConflicts(t *testing.T) {
	t.Parallel()
	coll := collection(t, options.Client())
	keys := bson.D{{Key: "rank", Value: 1}}
	builder := options.Index().SetName("rank")
	models := []mongo.IndexModel{{Keys: keys, Options: builder}}
	before := len(builder.List())

	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			if err := indexes.Ensure(t.Context(), coll, models); err != nil {
				t.Error(err)
			}
		})
	}

	group.Wait()

	if len(builder.List()) != before {
		t.Fatal("Ensure modified shared options")
	}

	for _, tc := range []struct {
		model mongo.IndexModel
		code  int
	}{
		{mongo.IndexModel{Keys: keys, Options: options.Index().SetName("alternate_rank")}, 85},
		{mongo.IndexModel{Keys: bson.D{{Key: "other", Value: 1}}, Options: options.Index().SetName("rank")}, 86},
	} {
		err := indexes.Ensure(t.Context(), coll, []mongo.IndexModel{tc.model})

		var server mongo.ServerError
		if !errors.As(err, &server) || !server.HasErrorCode(tc.code) {
			t.Fatalf("want server code %d, got %v", tc.code, err)
		}
	}

	if err := indexes.Ensure(t.Context(), coll, models); err != nil {
		t.Fatalf("original index changed: %v", err)
	}
}

func TestFindUsesPreparedBytesWithCustomRawEncoder(t *testing.T) {
	t.Parallel()

	reg := bson.NewRegistry()
	reg.RegisterTypeEncoder(reflect.TypeFor[bson.Raw](), bson.ValueEncoderFunc(func(bson.EncodeContext, bson.ValueWriter, reflect.Value) error { return errors.New("raw re-encoded") }))

	coll := collection(t, options.Client().SetRegistry(reg))
	if _, err := keyset.FindPage[bson.M](t.Context(), coll, nil, keyset.Query{Filter: bson.D{{Key: "rank", Value: 1}}, Registry: reg}); err != nil {
		t.Fatal(err)
	}
}

func TestMongoHistoricalContinuation(t *testing.T) {
	t.Parallel()
	coll := collection(t, options.Client())
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	signer, err := pagetoken.New(bytes.Repeat([]byte{0x42}, 32), pagetoken.WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}

	meta := bson.D{{Key: "a", Value: int32(1)}, {Key: "b", Value: int64(2)}}
	for _, seq := range []int64{41, 42, 43} {
		doc := bson.D{{Key: "meta", Value: meta}, {Key: "rank", Value: int32(7)}, {Key: "identity", Value: bson.D{{Key: "seq", Value: seq}}}}
		if _, err := coll.InsertOne(t.Context(), doc); err != nil {
			t.Fatal(err)
		}
	}

	q := keyset.Query{Filter: bson.D{{Key: "meta", Value: bson.M{"a": int32(1), "b": int64(2)}}}, Sort: bson.D{{Key: "rank", Value: -1}}, TiebreakPath: "identity.seq", Subject: "fixture-reader", TimestampPrecision: time.Millisecond}

	for _, order := range []string{"sorted", "reversed"} {
		data, err := os.ReadFile("../testdata/tokens-" + order + ".json")
		if err != nil {
			t.Fatal(err)
		}

		var fixture struct{ Token string }
		if err := json.Unmarshal(data, &fixture); err != nil {
			t.Fatal(err)
		}

		q.After = fixture.Token

		page, err := keyset.FindPage[bson.Raw](t.Context(), coll, signer, q)
		if order == "reversed" {
			if !errors.Is(err, pagetoken.ErrFilterChanged) {
				t.Fatalf("reversed: %v", err)
			}

			continue
		}

		if err != nil || len(page.Items) != 1 || page.Items[0].Lookup("identity", "seq").Int64() != 43 || page.Next != "" {
			t.Fatalf("historical continuation: %v, %v", page, err)
		}
	}
}
