// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package bsonraw checks the structure of raw BSON before anything else reads it. The driver trusts the lengths inside raw BSON
// it copies (bson.Raw, bson.RawValue, bsoncore.Document, the output of a bson.Marshaler): a document whose length is below its
// own header makes it slice out of range and panic. [Walk] reads a value once, without recursion, checking every length against
// the bytes that hold it, every string's terminator and every document's end, and bounds the documents it holds open at once, so
// a value it accepts is one the driver reads without running past its bounds.
//
// # Usage
//
//	err := bsonraw.Walk(bson.TypeEmbeddedDocument, raw, maxNesting, func(depth int, t bson.Type, key []byte) error {
//		return checkKey(key)
//	})
package bsonraw
