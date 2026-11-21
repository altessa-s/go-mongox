// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonbound

import (
	"bytes"
	"errors"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"

	filtereval "github.com/altessa-s/go-aip/filter/evaluator"
)

// ErrLimitExceeded fails an encoding that would exceed its byte limit, its nesting limit or its budget.
var ErrLimitExceeded = errors.New("mongo: BSON encoding exceeds its limit")

// encodeError keeps custom encoder diagnostics out of messages while preserving the cause.
type encodeError struct{ cause error }

func (e encodeError) Error() string { return "bsonbound: encoding failed" }
func (e encodeError) Unwrap() error { return e.cause }

// BSON sizes of the parts of an encoded value.
const (
	bsonLengthBytes   = 4                                 // the int32 length of a string, document or binary
	bsonDocumentBytes = bsonLengthBytes + 1               // the length and terminator of an empty document or array
	bsonElementBytes  = 2                                 // the type byte and the key terminator of an element
	bsonStringBytes   = bsonLengthBytes + 1               // the length and terminator of a string
	bsonBinaryBytes   = bsonLengthBytes + 1               // the length and subtype of a binary
	bsonObjectIDBytes = 12                                // an ObjectID
	bsonDecimalBytes  = 16                                // a Decimal128
	bsonWordBytes     = 8                                 // a double, int64, date time or timestamp
	bsonInt32Bytes    = 4                                 // an int32
	binaryOldSubtype  = bson.TypeBinaryBinaryOld          // the binary subtype that repeats its length
	bsonCodeWithScope = bsonLengthBytes + bsonStringBytes // the length and code string of a code with scope, without the code
)

// Bound measures the BSON a [NewValueWriter] forwards, charges it to a budget as it grows and refuses a value once the encoding
// would exceed its byte limit, the budget or its nesting limit.
type Bound struct {
	budget  filtereval.Budget // nil charges nothing
	limit   int
	nesting int
	size    int // BSON bytes forwarded so far
	charged int // units of size charged to budget
	depth   int // documents, arrays and scopes open
}

// Size returns the BSON bytes forwarded so far.
func (b *Bound) Size() int { return b.size }

// Depth returns the documents, arrays and scopes still open; zero once a whole value was written.
func (b *Bound) Depth() int { return b.depth }

// add accounts for n more bytes of BSON.
func (b *Bound) add(n int) error {
	if n > b.limit-b.size {
		return ErrLimitExceeded
	}

	b.size += n

	if b.budget == nil {
		return nil
	}

	if units := b.size/filtereval.EncodedBytesPerUnit - b.charged; units > 0 {
		if !b.budget.Charge(units) {
			return ErrLimitExceeded
		}

		b.charged += units
	}

	return nil
}

// open accounts for a document, array or scope of n bytes when empty.
func (b *Bound) open(n int) error {
	if b.depth >= b.nesting {
		return ErrLimitExceeded
	}

	if err := b.add(n); err != nil {
		return err
	}

	b.depth++

	return nil
}

// boundedValueWriter forwards to a driver [bson.ValueWriter] the values [Bound] admits.
type boundedValueWriter struct {
	vw    bson.ValueWriter
	bound *Bound
}

var _ bson.ValueWriter = boundedValueWriter{}

// NewValueWriter returns vw bounded to limit bytes of BSON and nesting open documents, arrays and scopes, charged to budget (nil:
// no budget) at one unit per [filtereval.EncodedBytesPerUnit] bytes.
func NewValueWriter(vw bson.ValueWriter, limit, nesting int, budget filtereval.Budget) (bson.ValueWriter, *Bound) {
	bound := &Bound{budget: budget, limit: limit, nesting: nesting}
	return boundedValueWriter{vw: vw, bound: bound}, bound
}

// Marshal encodes doc through a writer bounded to limit bytes and nesting levels, with the encoder configure prepares (a
// registry, encoder settings; nil leaves the driver's defaults). Unlike a limit on the buffer the driver writes the finished
// document to, the bound stops the encoder as soon as the running size passes it, with no more than limit bytes buffered.
func Marshal(doc any, limit, nesting int, configure func(*bson.Encoder)) (bson.Raw, error) {
	var buf bytes.Buffer

	vw, bound := NewValueWriter(bson.NewDocumentWriter(&buf), limit, nesting, nil)
	enc := bson.NewEncoder(vw)

	if configure != nil {
		configure(enc)
	}

	if err := enc.Encode(doc); err != nil {
		if errors.Is(err, ErrLimitExceeded) {
			return nil, ErrLimitExceeded
		}

		return nil, encodeError{err}
	}

	if bound.depth != 0 {
		return nil, ErrLimitExceeded
	}

	return buf.Bytes(), nil
}

func (w boundedValueWriter) WriteArray() (bson.ArrayWriter, error) {
	if err := w.bound.open(bsonDocumentBytes); err != nil {
		return nil, err
	}

	aw, err := w.vw.WriteArray()
	if err != nil {
		return nil, err
	}

	return &boundedArrayWriter{aw: aw, bound: w.bound}, nil
}

func (w boundedValueWriter) WriteBinary(b []byte) error {
	return w.WriteBinaryWithSubtype(b, bson.TypeBinaryGeneric)
}

func (w boundedValueWriter) WriteBinaryWithSubtype(b []byte, btype byte) error {
	n := bsonBinaryBytes + len(b)
	if btype == binaryOldSubtype {
		n += bsonLengthBytes
	}

	if err := w.bound.add(n); err != nil {
		return err
	}

	return w.vw.WriteBinaryWithSubtype(b, btype)
}

func (w boundedValueWriter) WriteBoolean(v bool) error {
	if err := w.bound.add(1); err != nil {
		return err
	}

	return w.vw.WriteBoolean(v)
}

func (w boundedValueWriter) WriteCodeWithScope(code string) (bson.DocumentWriter, error) {
	if err := w.bound.open(bsonCodeWithScope + len(code) + bsonDocumentBytes); err != nil {
		return nil, err
	}

	dw, err := w.vw.WriteCodeWithScope(code)
	if err != nil {
		return nil, err
	}

	return boundedDocumentWriter{dw: dw, bound: w.bound}, nil
}

func (w boundedValueWriter) WriteDBPointer(ns string, oid bson.ObjectID) error {
	if err := w.bound.add(bsonStringBytes + len(ns) + bsonObjectIDBytes); err != nil {
		return err
	}

	return w.vw.WriteDBPointer(ns, oid)
}

func (w boundedValueWriter) WriteDateTime(dt int64) error {
	if err := w.bound.add(bsonWordBytes); err != nil {
		return err
	}

	return w.vw.WriteDateTime(dt)
}

func (w boundedValueWriter) WriteDecimal128(d bson.Decimal128) error {
	if err := w.bound.add(bsonDecimalBytes); err != nil {
		return err
	}

	return w.vw.WriteDecimal128(d)
}

func (w boundedValueWriter) WriteDouble(f float64) error {
	if err := w.bound.add(bsonWordBytes); err != nil {
		return err
	}

	return w.vw.WriteDouble(f)
}

func (w boundedValueWriter) WriteInt32(i int32) error {
	if err := w.bound.add(bsonInt32Bytes); err != nil {
		return err
	}

	return w.vw.WriteInt32(i)
}

func (w boundedValueWriter) WriteInt64(i int64) error {
	if err := w.bound.add(bsonWordBytes); err != nil {
		return err
	}

	return w.vw.WriteInt64(i)
}

func (w boundedValueWriter) WriteJavascript(code string) error {
	if err := w.bound.add(bsonStringBytes + len(code)); err != nil {
		return err
	}

	return w.vw.WriteJavascript(code)
}

func (w boundedValueWriter) WriteMaxKey() error { return w.vw.WriteMaxKey() }

func (w boundedValueWriter) WriteMinKey() error { return w.vw.WriteMinKey() }

func (w boundedValueWriter) WriteNull() error { return w.vw.WriteNull() }

func (w boundedValueWriter) WriteObjectID(oid bson.ObjectID) error {
	if err := w.bound.add(bsonObjectIDBytes); err != nil {
		return err
	}

	return w.vw.WriteObjectID(oid)
}

func (w boundedValueWriter) WriteRegex(pattern, options string) error {
	if err := w.bound.add(len(pattern) + 1 + len(options) + 1); err != nil {
		return err
	}

	return w.vw.WriteRegex(pattern, options)
}

func (w boundedValueWriter) WriteString(s string) error {
	if err := w.bound.add(bsonStringBytes + len(s)); err != nil {
		return err
	}

	return w.vw.WriteString(s)
}

func (w boundedValueWriter) WriteDocument() (bson.DocumentWriter, error) {
	if err := w.bound.open(bsonDocumentBytes); err != nil {
		return nil, err
	}

	dw, err := w.vw.WriteDocument()
	if err != nil {
		return nil, err
	}

	return boundedDocumentWriter{dw: dw, bound: w.bound}, nil
}

func (w boundedValueWriter) WriteSymbol(symbol string) error {
	if err := w.bound.add(bsonStringBytes + len(symbol)); err != nil {
		return err
	}

	return w.vw.WriteSymbol(symbol)
}

func (w boundedValueWriter) WriteTimestamp(t, i uint32) error {
	if err := w.bound.add(bsonWordBytes); err != nil {
		return err
	}

	return w.vw.WriteTimestamp(t, i)
}

func (w boundedValueWriter) WriteUndefined() error { return w.vw.WriteUndefined() }

// boundedDocumentWriter forwards the elements of a document, charging each key before the driver writes it.
type boundedDocumentWriter struct {
	dw    bson.DocumentWriter
	bound *Bound
}

func (w boundedDocumentWriter) WriteDocumentElement(key string) (bson.ValueWriter, error) {
	if err := w.bound.add(bsonElementBytes + len(key)); err != nil {
		return nil, err
	}

	vw, err := w.dw.WriteDocumentElement(key)
	if err != nil {
		return nil, err
	}

	return boundedValueWriter{vw: vw, bound: w.bound}, nil
}

func (w boundedDocumentWriter) WriteDocumentEnd() error {
	if err := w.dw.WriteDocumentEnd(); err != nil {
		return err
	}

	w.bound.depth--

	return nil
}

// boundedArrayWriter forwards the elements of an array, charging each index key before the driver writes it.
type boundedArrayWriter struct {
	aw    bson.ArrayWriter
	bound *Bound
	next  int // the index of the next element
}

func (w *boundedArrayWriter) WriteArrayElement() (bson.ValueWriter, error) {
	if err := w.bound.add(bsonElementBytes + len(strconv.Itoa(w.next))); err != nil {
		return nil, err
	}

	vw, err := w.aw.WriteArrayElement()
	if err != nil {
		return nil, err
	}

	w.next++

	return boundedValueWriter{vw: vw, bound: w.bound}, nil
}

func (w *boundedArrayWriter) WriteArrayEnd() error {
	if err := w.aw.WriteArrayEnd(); err != nil {
		return err
	}

	w.bound.depth--

	return nil
}
