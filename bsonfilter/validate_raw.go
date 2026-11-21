// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"errors"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-mongox/bsonraw"
)

// rawDocument checks the document or array doc, reached at depth, element by element ([bsonraw.Walk]), charging each to the walk:
// the size is bounded before anything is read, the structure is checked as it is read, every key is checked against the
// deny-list and every type against those [Validate] admits.
func (w *filterWalk) rawDocument(doc []byte, depth int) error {
	return w.rawValue(bson.TypeEmbeddedDocument, doc, depth)
}

// rawValue checks a raw value of type t, reached at depth: a document or an array as [filterWalk.rawDocument] does, any other value
// must be a whole value of an admitted BSON type. The size is bounded before anything is read.
func (w *filterWalk) rawValue(t bson.Type, data []byte, depth int) error {
	switch {
	case depth > maxFilterNesting:
		return ErrTooDeep
	case len(data) > maxFilterBytes:
		return ErrTooLarge
	}

	if err := admitRawType(t); err != nil {
		return err
	}

	// The document reached at depth is open with the documents inside it, up to maxFilterNesting in all.
	err := bsonraw.Walk(t, data, maxFilterNesting-depth+1, func(_ int, et bson.Type, k []byte) error {
		if err := w.take(1); err != nil {
			return err
		}

		if err := rawKey(k); err != nil {
			return err
		}

		return admitRawType(et)
	})

	switch {
	case errors.Is(err, bsonraw.ErrMalformed):
		return ErrMalformedRaw
	case errors.Is(err, bsonraw.ErrTooDeep):
		return ErrTooDeep
	default:
		return err
	}
}

// rawKey fails for a deny-listed key of raw BSON. Every deny-listed operator starts with "$", so any other key is accepted
// without converting it to a string, a conversion that would allocate for every key of every filter [Prepare] checks.
func rawKey(k []byte) error {
	if len(k) == 0 || k[0] != '$' {
		return nil
	}

	return key(string(k))
}

// admitRawType fails with [ErrValueType] for a raw BSON type a filter must not hold: server-side code (JavaScript, with or without
// a scope), the deprecated symbol and DBPointer types and unknown types.
func admitRawType(t bson.Type) error {
	switch t { //nolint:exhaustive // the remaining types are refused below
	case bson.TypeNull, bson.TypeUndefined, bson.TypeMinKey, bson.TypeMaxKey, bson.TypeBoolean, bson.TypeInt32, bson.TypeDouble,
		bson.TypeInt64, bson.TypeDateTime, bson.TypeTimestamp, bson.TypeObjectID, bson.TypeDecimal128, bson.TypeString,
		bson.TypeBinary, bson.TypeEmbeddedDocument, bson.TypeArray, bson.TypeRegex:
		return nil
	default:
		return fmt.Errorf("%w: raw BSON type %s", ErrValueType, t)
	}
}
