// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package keyset_test

import (
	"errors"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/altessa-s/go-aip/pagination/pagetoken"

	"github.com/altessa-s/go-mongox/keyset"
)

type tokenCodecFake struct{ err error }

func (f tokenCodecFake) Issue([]byte, pagetoken.Bindings) (string, error) {
	return "", f.err
}

func (f tokenCodecFake) Resolve(string, pagetoken.Bindings) ([]byte, error) {
	return nil, f.err
}

func TestFindPage_UsesTheSigner(t *testing.T) {
	t.Parallel()

	boom := errors.New("signer returned token contents")

	_, err := keyset.FindPage[bson.M](t.Context(), nil, tokenCodecFake{err: boom}, keyset.Query{After: "token"})
	if !errors.Is(err, boom) || strings.Contains(err.Error(), boom.Error()) {
		t.Fatalf("got %v, want %v", err, boom)
	}

	_, err = keyset.FindPage[bson.M](t.Context(), nil, (*pagetoken.Codec)(nil), keyset.Query{After: "token"})
	if !errors.Is(err, keyset.ErrTokensRequired) || !errors.Is(err, pagetoken.ErrCodecRequired) || pagetoken.Classify(err) != pagetoken.FailureMalformed {
		t.Fatalf("got %v, want %v", err, keyset.ErrTokensRequired)
	}
}
