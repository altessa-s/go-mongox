// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package bsonfilter

import (
	"context"
	"maps"
	"reflect"
	"slices"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// canonicalizer is one canonicalization pass: the context it stops with, the error it stopped with and the canonical form of
// every map it met, by identity.
type canonicalizer struct {
	ctx        context.Context //nolint:containedctx // the pass lives for one call
	untilCheck int
	err        error
	maps       map[mapKey]canonical
}

// mapKey identifies a map by its type and the map it refers to, which the filter keeps alive for the pass.
type mapKey struct {
	t reflect.Type
	p uintptr
}

// canonical is the canonical form of a value and whether it differs from the value.
type canonical struct {
	value   any
	changed bool
}

// stopped reports whether the pass has stopped, checking the context every [cancelCheckInterval] calls.
func (c *canonicalizer) stopped() bool {
	if c.err != nil {
		return true
	}

	if c.untilCheck--; c.untilCheck <= 0 {
		c.untilCheck = cancelCheckInterval

		if c.ctx != nil {
			c.err = c.ctx.Err()
		}
	}

	return c.err != nil
}

// value returns v canonical, and whether it differs from v.
func (c *canonicalizer) value(v any) (any, bool) {
	if c.stopped() {
		return v, false
	}

	switch v := v.(type) {
	case nil, bson.Raw, bson.RawArray, bson.RawValue:
		return v, false
	case bson.D:
		return c.d(v)
	case []bson.E:
		if d, changed := c.d(v); changed {
			return d, true
		}

		return v, false
	case bson.E:
		value, changed := c.value(v.Value)

		return bson.E{Key: v.Key, Value: value}, changed
	case bson.M:
		return canonicalMap(c, v)
	case map[string]any:
		return canonicalMap(c, v)
	case bson.A:
		return canonicalList(c, v)
	case []any:
		return canonicalList(c, v)
	case []bson.D:
		return canonicalList(c, v)
	case []bson.M:
		return canonicalList(c, v)
	default:
		return c.other(v)
	}
}

// d returns d with its values canonical, and whether one differs.
func (c *canonicalizer) d(d bson.D) (bson.D, bool) {
	var out bson.D

	for i, e := range d {
		value, changed := c.value(e.Value)
		if changed && out == nil {
			out = slices.Clone(d)
		}

		if out != nil {
			out[i].Value = value
		}
	}

	if out == nil {
		return d, false
	}

	return out, true
}

// canonicalMap returns m as a bson.D in key order when it has two or more entries or a value that changes. A map met again
// returns the form it got the first time.
func canonicalMap[M ~map[string]any](c *canonicalizer, m M) (any, bool) {
	k := mapKey{t: reflect.TypeOf(m), p: reflect.ValueOf(m).Pointer()}
	if r, ok := c.maps[k]; ok {
		return r.value, r.changed
	}

	changed := len(m) > 1
	values := make(map[string]any, len(m))

	for k, v := range m {
		value, ch := c.value(v)
		values[k] = value
		changed = changed || ch
	}

	r := canonical{value: m}

	if changed {
		d := make(bson.D, 0, len(m))
		for _, k := range slices.Sorted(maps.Keys(values)) {
			d = append(d, bson.E{Key: k, Value: values[k]})
		}

		r = canonical{value: d, changed: true}
	}

	if c.maps == nil {
		c.maps = make(map[mapKey]canonical)
	}

	c.maps[k] = r

	return r.value, r.changed
}

// canonicalList returns s as a bson.A of canonical elements when one differs.
func canonicalList[S ~[]E, E any](c *canonicalizer, s S) (any, bool) {
	var out bson.A

	for i, e := range s {
		value, changed := c.value(e)
		if changed && out == nil {
			out = make(bson.A, len(s))
			for j := range i {
				out[j] = s[j]
			}
		}

		if out != nil {
			out[i] = value
		}
	}

	if out == nil {
		return s, false
	}

	return out, true
}

// other returns an unnamed slice of containers or lists ([]map[string]any, [][]any, …) as a bson.A of canonical elements when
// one differs; a leaf is returned as it is.
func (c *canonicalizer) other(v any) (any, bool) {
	rv := reflect.ValueOf(v)

	t := rv.Type()
	if t.Kind() != reflect.Slice || t.Name() != "" || (!containerTypes.Contains(t.Elem()) && t.Elem().Kind() != reflect.Slice) {
		return v, false
	}

	var out bson.A

	for i := range rv.Len() {
		value, changed := c.value(rv.Index(i).Interface())
		if changed && out == nil {
			out = make(bson.A, rv.Len())
			for j := range i {
				out[j] = rv.Index(j).Interface()
			}
		}

		if out != nil {
			out[i] = value
		}
	}

	if out == nil {
		return v, false
	}

	return out, true
}
