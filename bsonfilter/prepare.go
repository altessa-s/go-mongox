// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/altessa-s/go-mongox/bsonbound"
)

// Prepare returns owned BSON for execution and token binding. It checks both
// the Go input and the encoder output. Ordered documents and raw keep their order.
func Prepare(ctx context.Context, doc any, reg *bson.Registry, opts *options.BSONOptions) (bson.Raw, error) {
	switch d := doc.(type) {
	case nil:
		doc = bson.D{}
	case bson.D:
		if d == nil {
			doc = bson.D{}
		}
	case bson.M:
		if d == nil {
			doc = bson.D{}
		}
	case map[string]any:
		if d == nil {
			doc = bson.D{}
		}
	case bson.Raw:
		if len(d) > maxFilterBytes {
			return nil, ErrTooLarge
		}

		raw := bson.Raw(bytes.Clone(d))

		return checkedRaw(ctx, raw)
	default:
		return nil, ErrValueType
	}

	if err := Validate(ctx, doc, reg); err != nil {
		return nil, err
	}

	c := canonicalizer{ctx: ctx, untilCheck: cancelCheckInterval}

	canonical, _ := c.value(doc)
	if c.err != nil {
		return nil, c.err
	}

	return marshalFilter(ctx, canonical, reg, opts)
}

func checkedRaw(ctx context.Context, raw bson.Raw) (bson.Raw, error) {
	walk := newFilterWalk(ctx, nil)
	if err := walk.rawDocument(raw, 0); err != nil {
		return nil, err
	}

	return raw, nil
}

// marshalFilter checks the bytes produced by the configured encoder.
func marshalFilter(ctx context.Context, filter any, reg *bson.Registry, opts *options.BSONOptions) (bson.Raw, error) {
	raw, err := bsonbound.Marshal(filter, maxFilterBytes, encodeNesting, func(enc *bson.Encoder) {
		if reg != nil {
			enc.SetRegistry(reg)
		}

		applyBSONOptions(enc, opts)
	})
	if err != nil {
		if errors.Is(err, bsonbound.ErrLimitExceeded) {
			return nil, fmt.Errorf("%w: %w", ErrTooLarge, bsonbound.ErrLimitExceeded)
		}

		return nil, err
	}

	return checkedRaw(ctx, raw)
}

// applyBSONOptions configures enc with the encoder settings of opts, as the driver does for a collection.
func applyBSONOptions(enc *bson.Encoder, opts *options.BSONOptions) {
	if opts == nil {
		return
	}

	for _, o := range []struct {
		on    bool
		apply func()
	}{
		{opts.ErrorOnInlineDuplicates, enc.ErrorOnInlineDuplicates},
		{opts.IntMinSize, enc.IntMinSize},
		{opts.NilByteSliceAsEmpty, enc.NilByteSliceAsEmpty},
		{opts.NilMapAsEmpty, enc.NilMapAsEmpty},
		{opts.NilSliceAsEmpty, enc.NilSliceAsEmpty},
		{opts.OmitZeroStruct, enc.OmitZeroStruct},
		{opts.OmitEmpty, enc.OmitEmpty},
		{opts.StringifyMapKeysWithFmt, enc.StringifyMapKeysWithFmt},
		{opts.UseJSONStructTags, enc.UseJSONStructTags},
	} {
		if o.on {
			o.apply()
		}
	}
}
