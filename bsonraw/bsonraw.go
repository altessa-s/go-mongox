// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonraw

import (
	"bytes"
	"encoding/binary"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Sizes of the parts of the BSON wire format.
const (
	// LengthBytes is the size of the int32 length prefix of a document, an array, a string or a binary.
	LengthBytes = 4
	// MinDocumentBytes is the size of an empty document: its length and its terminator.
	MinDocumentBytes = LengthBytes + 1

	int64Bytes      = 8  // a double, a date, an int64 or a timestamp
	objectIDBytes   = 12 // an ObjectID, also the tail of a DBPointer
	decimal128Bytes = 16
	// minCodeWithScopeBytes is a code with scope holding an empty code string and an empty scope document.
	minCodeWithScopeBytes = LengthBytes + LengthBytes + 1 + MinDocumentBytes
	// binaryOld is the deprecated binary subtype whose data repeats its length.
	binaryOld = 0x02
)

var (
	// ErrMalformed fails raw BSON that is not well formed: a length beyond the bytes that hold it or below its own header, a string
	// or a key without its terminator, a document that does not end where its length says, a boolean other than 0 or 1, an unknown
	// type.
	ErrMalformed = errors.New("bsonraw: malformed raw BSON")
	// ErrTooDeep fails raw BSON that holds more documents open at once than the bound [Walk] is given.
	ErrTooDeep = errors.New("bsonraw: raw BSON nests too deep")
)

// Visit is called by [Walk] for every element of every document and array, before its value is read: depth is the number of
// documents open around it (1 for an element of the outermost one), t its type and key its key, which aliases the walked bytes.
// A non-nil error stops the walk, which returns it as is.
type Visit func(depth int, t bson.Type, key []byte) error

// Walk checks that data is exactly one well-formed BSON value of type t: every length within the bytes that hold it and not below
// its own header, every string, key and regular expression terminated, every document and array (the scope of a code with scope
// included) ending with its terminator where its length says, an old binary repeating its length, every boolean 0 or 1, and no
// unknown type. It holds
// at most maxDepth documents open at once (an array and a scope count as documents), failing with [ErrTooDeep] beyond; any other
// defect fails with [ErrMalformed]. visit, which may be nil, is called for every element. Walk reads data once, without recursion,
// in time linear in its length.
func Walk(t bson.Type, data []byte, maxDepth int, visit Visit) error {
	s := scanner{data: data, maxDepth: maxDepth}
	s.ends = s.inline[:0]

	var err error

	switch t {
	case bson.TypeEmbeddedDocument, bson.TypeArray:
		err = s.open()
	case bson.TypeCodeWithScope:
		err = s.codeWithScope()
	default:
		err = s.skip(t, len(data))
	}

	if err != nil {
		return err
	}

	if err := s.elements(visit); err != nil {
		return err
	}

	if s.pos != len(data) {
		return ErrMalformed
	}

	return nil
}

// scanner reads the structure of a BSON value: pos is the offset read next, ends the offsets of the terminators of the open
// documents, innermost last.
type scanner struct {
	data     []byte
	pos      int
	maxDepth int
	ends     []int
	inline   [16]int // the first ends, without an allocation
}

// elements reads the elements of the open documents until all of them are closed.
func (s *scanner) elements(visit Visit) error {
	for len(s.ends) > 0 {
		end := s.ends[len(s.ends)-1]
		if s.pos == end { // the terminator of the innermost open document
			if s.data[s.pos] != 0 {
				return ErrMalformed
			}

			s.pos++
			s.ends = s.ends[:len(s.ends)-1]

			continue
		}

		t := bson.Type(s.data[s.pos])

		key := bytes.IndexByte(s.data[s.pos+1:end], 0)
		if key < 0 {
			return ErrMalformed
		}

		if visit != nil {
			if err := visit(len(s.ends), t, s.data[s.pos+1:s.pos+1+key]); err != nil {
				return err
			}
		}

		s.pos += 1 + key + 1

		var err error

		switch t {
		case bson.TypeEmbeddedDocument, bson.TypeArray:
			err = s.open()
		case bson.TypeCodeWithScope:
			err = s.codeWithScope()
		default:
			err = s.skip(t, end)
		}

		if err != nil {
			return err
		}
	}

	return nil
}

// int32At returns the little-endian int32 at offset i, or -1 when data ends before it.
func (s *scanner) int32At(i int) int {
	if i < 0 || i > len(s.data)-LengthBytes {
		return -1
	}

	return int(int32(binary.LittleEndian.Uint32(s.data[i:]))) //nolint:gosec // a BSON length is a signed int32
}

// limit is the offset of the terminator of the innermost open document, before which a value inside it ends, or the end of data.
func (s *scanner) limit() int {
	if len(s.ends) == 0 {
		return len(s.data)
	}

	return s.ends[len(s.ends)-1]
}

// open enters the document or array at pos.
func (s *scanner) open() error {
	if len(s.ends) >= s.maxDepth {
		return ErrTooDeep
	}

	n := s.int32At(s.pos)
	if n < MinDocumentBytes || n > s.limit()-s.pos {
		return ErrMalformed
	}

	s.ends = append(s.ends, s.pos+n-1)
	s.pos += LengthBytes

	return nil
}

// codeWithScope enters the scope document of the code with scope at pos, after checking its code string; the scope ends where the
// code with scope does.
func (s *scanner) codeWithScope() error {
	total, code := s.int32At(s.pos), s.int32At(s.pos+LengthBytes)
	if total < minCodeWithScopeBytes || total > s.limit()-s.pos || code < 1 || code > total-2*LengthBytes-MinDocumentBytes {
		return ErrMalformed
	}

	scope := s.pos + 2*LengthBytes + code
	if s.data[scope-1] != 0 || s.int32At(scope) != s.pos+total-scope {
		return ErrMalformed
	}

	s.pos = scope

	return s.open()
}

// skip moves past a value of type t that holds no document, which ends at or before end.
func (s *scanner) skip(t bson.Type, end int) error {
	var n int

	switch t {
	case bson.TypeUndefined, bson.TypeNull, bson.TypeMinKey, bson.TypeMaxKey:
	case bson.TypeBoolean:
		if s.pos >= end || s.data[s.pos] > 1 {
			return ErrMalformed
		}

		n = 1
	case bson.TypeInt32:
		n = LengthBytes
	case bson.TypeDouble, bson.TypeDateTime, bson.TypeInt64, bson.TypeTimestamp:
		n = int64Bytes
	case bson.TypeObjectID:
		n = objectIDBytes
	case bson.TypeDecimal128:
		n = decimal128Bytes
	case bson.TypeString, bson.TypeJavaScript, bson.TypeSymbol, bson.TypeDBPointer:
		return s.skipString(t, end)
	case bson.TypeBinary:
		return s.skipBinary(end)
	case bson.TypeRegex:
		return s.skipRegex(end)
	default:
		return ErrMalformed
	}

	if n > end-s.pos {
		return ErrMalformed
	}

	s.pos += n

	return nil
}

// skipString moves past a string, JavaScript code, a symbol or a DB pointer (a string and an ObjectID), checking the terminator
// the string's length counts.
func (s *scanner) skipString(t bson.Type, end int) error {
	l := s.int32At(s.pos)
	if l < 1 || l > end-s.pos-LengthBytes {
		return ErrMalformed
	}

	n := LengthBytes + l
	if s.data[s.pos+n-1] != 0 {
		return ErrMalformed
	}

	if t == bson.TypeDBPointer {
		if objectIDBytes > end-s.pos-n {
			return ErrMalformed
		}

		n += objectIDBytes
	}

	s.pos += n

	return nil
}

// skipBinary moves past a binary: its length, its subtype and its data, which for the old binary subtype repeats the length of the
// rest.
func (s *scanner) skipBinary(end int) error {
	l := s.int32At(s.pos)
	if l < 0 || l > end-s.pos-LengthBytes-1 {
		return ErrMalformed
	}

	if s.data[s.pos+LengthBytes] == binaryOld && (l < LengthBytes || s.int32At(s.pos+LengthBytes+1) != l-LengthBytes) {
		return ErrMalformed
	}

	s.pos += LengthBytes + 1 + l

	return nil
}

// skipRegex moves past a regular expression: its pattern and its options, both terminated.
func (s *scanner) skipRegex(end int) error {
	if s.pos > end {
		return ErrMalformed
	}

	pattern := bytes.IndexByte(s.data[s.pos:end], 0)
	if pattern < 0 {
		return ErrMalformed
	}

	options := bytes.IndexByte(s.data[s.pos+pattern+1:end], 0)
	if options < 0 {
		return ErrMalformed
	}

	s.pos += pattern + 1 + options + 1

	return nil
}
