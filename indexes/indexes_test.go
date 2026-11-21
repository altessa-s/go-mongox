// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package indexes_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/altessa-s/go-mongox/indexes"

	mongooptions "go.mongodb.org/mongo-driver/v2/mongo/options"
)

func named(name string, keys any) mongo.IndexModel {
	return mongo.IndexModel{Keys: keys, Options: mongooptions.Index().SetName(name)}
}

// TestEnsureRejectsInvalidModels: invalid models are rejected before any server call (the collection is nil).
func TestEnsureRejectsInvalidModels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		models []mongo.IndexModel
	}{
		{name: "duplicate explicit names", models: []mongo.IndexModel{
			named("idx", bson.D{{Key: "a", Value: 1}}), named("idx", bson.D{{Key: "b", Value: 1}}),
		}},
		{name: "explicit name equals a generated one", models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "a", Value: 1}}}, named("a_1", bson.D{{Key: "b", Value: 1}}),
		}},
		{name: "empty explicit name", models: []mongo.IndexModel{named("", bson.D{{Key: "a", Value: 1}})}},
		{name: "double key value", models: []mongo.IndexModel{{Keys: bson.D{{Key: "a", Value: 1.0}}}}},
		{name: "empty keys", models: []mongo.IndexModel{{Keys: bson.D{}}}},
		{name: "keys that do not encode", models: []mongo.IndexModel{{Keys: func() {}}}},
		{name: "named keys that do not encode", models: []mongo.IndexModel{named("idx", func() {})}},
		{name: "named empty keys", models: []mongo.IndexModel{named("idx", bson.D{})}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := indexes.Ensure(t.Context(), nil, tc.models)
			require.ErrorIs(t, err, indexes.ErrInvalidIndexModels)
		})
	}
}

// TestEnsureWithoutModels sends nothing: the collection is never touched.
func TestEnsureWithoutModels(t *testing.T) {
	t.Parallel()

	require.NoError(t, indexes.Ensure(t.Context(), nil, nil))
}

// TestNames resolves explicit names and the names the driver generates from int32, int64 and string key values.
func TestNames(t *testing.T) {
	t.Parallel()

	names, err := indexes.Names([]mongo.IndexModel{
		named("idx_a", bson.D{{Key: "a", Value: 1}}),
		{Keys: bson.D{{Key: "b", Value: int32(1)}, {Key: "c", Value: int64(-1)}}},
		{Keys: bson.D{{Key: "body", Value: "text"}}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"idx_a", "b_1_c_-1", "body_text"}, names)
}

func TestNamesPreservesSetterFailure(t *testing.T) {
	t.Parallel()

	cause := errors.New("index option failed")
	builder := mongooptions.Index()
	builder.Opts = append(builder.Opts, func(*mongooptions.IndexOptions) error { return cause })

	_, err := indexes.Names([]mongo.IndexModel{{Options: builder}})
	require.ErrorIs(t, err, cause)
	require.ErrorIs(t, err, indexes.ErrInvalidIndexModels)
}
