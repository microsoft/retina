// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

var errNoURL = errors.New("no URL here")

func TestRedactURLQueries(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "url.Error from a failed request",
			in:   `Put "https://acct.blob.core.windows.net/captures/f.tar.gz?sig=SECRETSIGNATURE&sv=2022-11-02": dial tcp: lookup acct.blob.core.windows.net: no such host`,
			want: `Put "https://acct.blob.core.windows.net/captures/f.tar.gz?REDACTED": dial tcp: lookup acct.blob.core.windows.net: no such host`,
		},
		{
			name: "unquoted URL followed by text",
			in:   "GET https://acct.blob.core.windows.net/c?sv=1&sig=SECRETSIGNATURE failed",
			want: "GET https://acct.blob.core.windows.net/c?REDACTED failed",
		},
		{
			name: "several URLs",
			in:   "a http://h1/p?k=SECRET1 b https://h2?SECRET2",
			want: "a http://h1/p?REDACTED b https://h2?REDACTED",
		},
		{
			name: "fragment is kept",
			in:   "https://h/p?sig=SECRETSIGNATURE#frag",
			want: "https://h/p?REDACTED#frag",
		},
		{
			name: "SAS signature outside a URL",
			in:   "token sv=2022-11-02&sig=SECRETSIGNATURE&se=2026",
			want: "token sv=2022-11-02&sig=REDACTED&se=2026",
		},
		{
			name: "SAS signature, uppercase",
			in:   "SIG=SECRETSIGNATURE",
			want: "sig=REDACTED",
		},
		{
			name: "query value echoed by the legacy storage SDK",
			in:   "storage: service returned error: StatusCode=400, ErrorCode=InvalidQueryParameterValue, QueryParameterName=sig, QueryParameterValue=SECRET+SIGNATURE/=",
			want: "storage: service returned error: StatusCode=400, ErrorCode=InvalidQueryParameterValue, QueryParameterName=sig, QueryParameterValue=REDACTED",
		},
		{
			name: "query value echoed in an XML response body",
			in:   "ERROR CODE: InvalidQueryParameterValue\n<Error><QueryParameterName>sig</QueryParameterName><QueryParameterValue>SECRET+SIGNATURE/=</QueryParameterValue></Error>\n",
			want: "ERROR CODE: InvalidQueryParameterValue\n<Error><QueryParameterName>sig</QueryParameterName><QueryParameterValue>REDACTED</QueryParameterValue></Error>\n",
		},
		{
			name: "empty query value is unchanged",
			in:   "StatusCode=403, QueryParameterName=, QueryParameterValue=",
			want: "StatusCode=403, QueryParameterName=, QueryParameterValue=",
		},
		{
			name: "URL without query is unchanged",
			in:   `Put "https://acct.blob.core.windows.net/captures/f.tar.gz": EOF`,
			want: `Put "https://acct.blob.core.windows.net/captures/f.tar.gz": EOF`,
		},
		{
			name: "question mark outside a URL is unchanged",
			in:   "is the file there? no",
			want: "is the file there? no",
		},
		{
			name: "word ending in sig is unchanged",
			in:   "configsig=value",
			want: "configsig=value",
		},
	}

	for i := range cases {
		tt := &cases[i]
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, RedactURLQueries(tt.in))
		})
	}
}

func TestRedactURLQueriesInError(t *testing.T) {
	require.NoError(t, RedactURLQueriesInError(nil))

	plain := errNoURL
	require.Same(t, plain, RedactURLQueriesInError(plain), "errors without a URL query are returned as is")

	urlErr := &url.Error{
		Op:  "Put",
		URL: "https://acct.blob.core.windows.net/captures/f.tar.gz?sig=SECRETSIGNATURE",
		Err: context.DeadlineExceeded,
	}
	err := RedactURLQueriesInError(fmt.Errorf("upload failed: %w", urlErr))
	require.EqualError(t, err, `upload failed: Put "https://acct.blob.core.windows.net/captures/f.tar.gz?REDACTED": context deadline exceeded`)
	require.ErrorIs(t, err, context.DeadlineExceeded, "the original error chain is kept")
	var gotURLErr *url.Error
	require.NotErrorAs(t, err, &gotURLErr, "the original error can't be reached")
	require.NoError(t, errors.Unwrap(err), "the original error can't be reached")

	// Wrapping the redacted error doesn't bring the original message back.
	require.NotContains(t, fmt.Errorf("output: %w", err).Error(), "SECRETSIGNATURE")
}
