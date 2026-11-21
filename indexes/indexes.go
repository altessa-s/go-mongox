// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package indexes

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	mongooptions "go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ErrInvalidIndexModels is returned by [Ensure] for models whose names collide or whose keys a name cannot be derived from.
var ErrInvalidIndexModels = errors.New("indexes: invalid index models")

// operationError hides index specifications and partial filter values in server and codec errors.
type operationError struct {
	message string
	cause   error
}

func (e operationError) Error() string { return e.message }
func (e operationError) Unwrap() error { return e.cause }

// Ensure creates the indexes of models on coll; it is idempotent and safe to run at every start. It is one createIndexes
// command: an index that already exists with the same specification is a no-op, and one that conflicts (same name with other
// keys or options, or the same keys under another name) fails it with the server's error, which is never resolved by dropping
// an index.
//
// The keys of every model are encoded once, with the driver's default registry, and those bytes are both the source of the
// name and the keys sent to the server, so a codec of the collection cannot make them differ. A model without a name gets the
// name the driver would give those keys ("field_1_other_-1"). Models must have distinct names and keys that encode to a
// non-empty document ([ErrInvalidIndexModels]).
func Ensure(ctx context.Context, coll *mongo.Collection, models []mongo.IndexModel) error {
	names, keys, err := resolve(models)
	if err != nil {
		return err
	}

	if len(models) == 0 {
		return nil
	}

	if _, err = coll.Indexes().CreateMany(ctx, namedModels(models, names, keys)); err != nil {
		return operationError{"indexes: create indexes failed", err}
	}

	return nil
}

// Names resolves the name of every model, as [Ensure] does, and checks they are distinct.
func Names(models []mongo.IndexModel) ([]string, error) {
	names, _, err := resolve(models)

	return names, err
}

// resolve encodes the keys of every model and resolves its name from them, checking the names are distinct.
func resolve(models []mongo.IndexModel) ([]string, []bson.Raw, error) {
	names := make([]string, len(models))
	keys := make([]bson.Raw, len(models))

	for i, model := range models {
		name, raw, err := modelName(model)
		if err != nil {
			return nil, nil, err
		}

		if slices.Contains(names[:i], name) {
			return nil, nil, fmt.Errorf("%w: duplicate index name %q", ErrInvalidIndexModels, name)
		}

		names[i], keys[i] = name, raw
	}

	return names, keys, nil
}

// namedModels returns copies of models carrying their encoded keys and their resolved names in options of their own: the driver
// writes the name into a model's options builder, which would race when callers share one model slice across goroutines (a
// package-level var).
func namedModels(models []mongo.IndexModel, names []string, keys []bson.Raw) []mongo.IndexModel {
	out := make([]mongo.IndexModel, len(models))

	for i, model := range models {
		opts := mongooptions.Index()
		if model.Options != nil {
			opts.Opts = slices.Clone(model.Options.Opts)
		}

		out[i] = mongo.IndexModel{Keys: keys[i], Options: opts.SetName(names[i])}
	}

	return out
}

// resolveIndexOptions applies the option setters of model (none for nil options) and returns the resulting index options.
func resolveIndexOptions(model mongo.IndexModel) (mongooptions.IndexOptions, error) {
	var args mongooptions.IndexOptions
	if model.Options == nil {
		return args, nil
	}

	for _, set := range model.Options.List() {
		if err := set(&args); err != nil {
			return mongooptions.IndexOptions{}, err
		}
	}

	return args, nil
}

// modelName returns the encoded keys of model and its name: the explicit one, or the one the driver generates, every key and
// its value (an integer or a string) joined by "_".
func modelName(model mongo.IndexModel) (string, bson.Raw, error) {
	args, err := resolveIndexOptions(model)
	if err != nil {
		return "", nil, operationError{"indexes: invalid index options", errors.Join(ErrInvalidIndexModels, err)}
	}

	keys, err := bson.Marshal(model.Keys)
	if err != nil {
		return "", nil, operationError{"indexes: invalid index keys", errors.Join(ErrInvalidIndexModels, err)}
	}

	elems, err := bson.Raw(keys).Elements()
	if err != nil || len(elems) == 0 {
		return "", nil, fmt.Errorf("%w: index keys must be a non-empty document", ErrInvalidIndexModels)
	}

	if args.Name != nil {
		if *args.Name == "" {
			return "", nil, fmt.Errorf("%w: empty index name", ErrInvalidIndexModels)
		}

		return *args.Name, keys, nil
	}

	parts := make([]string, 0, 2*len(elems))

	for _, e := range elems {
		v := e.Value()

		var s string

		switch v.Type {
		case bson.TypeInt32:
			s = strconv.FormatInt(int64(v.Int32()), 10)
		case bson.TypeInt64:
			s = strconv.FormatInt(v.Int64(), 10)
		case bson.TypeString:
			s = v.StringValue()
		default:
			return "", nil, fmt.Errorf("%w: key %q has a %v value", ErrInvalidIndexModels, e.Key(), v.Type)
		}

		parts = append(parts, e.Key(), s)
	}

	return strings.Join(parts, "_"), keys, nil
}
