// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

package keyset_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
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
	require.ErrorIs(t, err, boom)
	require.NotContains(t, err.Error(), boom.Error())

	_, err = keyset.FindPage[bson.M](t.Context(), nil, (*pagetoken.Codec)(nil), keyset.Query{After: "token"})
	require.ErrorIs(t, err, keyset.ErrTokensRequired)
	require.ErrorIs(t, err, pagetoken.ErrCodecRequired)
	require.Equal(t, pagetoken.FailureMalformed, pagetoken.Classify(err))
}

func TestFindPageRejectsTokenWithoutCodec(t *testing.T) {
	t.Parallel()

	codec, err := pagetoken.New(bytes.Repeat([]byte{1}, pagetoken.MinKeyLength))
	require.NoError(t, err)

	token, err := codec.Issue([]byte("x"), pagetoken.Bindings{})
	require.NoError(t, err)

	_, err = keyset.FindPage[bson.M](t.Context(), nil, nil, keyset.Query{After: token})
	require.ErrorIs(t, err, keyset.ErrTokensRequired)
	require.ErrorIs(t, err, pagetoken.ErrCodecRequired)
	require.Equal(t, pagetoken.FailureMalformed, pagetoken.Classify(err))
}
