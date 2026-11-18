// Copyright (c) Microsoft Corporation.
// Licensed under the MIT license.

package utils

import (
	"errors"
	"regexp"
)

const redacted = "REDACTED"

var (
	// The query string of a URL, e.g. the SAS token of a blob URL. Go's HTTP client includes the full request URL
	// in errors for failures that happen before a response (DNS, connection, TLS, timeout).
	urlQueryPattern = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^\s"'?#]*)\?[^\s"'#]*`)
	// A SAS signature outside a URL, e.g. a query string printed on its own.
	sasSignaturePattern = regexp.MustCompile(`(?i)\bsig=[^&\s"'#]+`)
	// The query value a storage service error rejected, which can be the SAS signature. The legacy storage SDK prints
	// it as the last field of its message; azblob prints the raw XML response body.
	queryParameterValuePattern    = regexp.MustCompile(`\bQueryParameterValue=[^\r\n]+`)
	queryParameterValueXMLPattern = regexp.MustCompile(`(?s)<QueryParameterValue>.+?</QueryParameterValue>`)
)

// RedactURLQueries replaces the query string of every URL in s, any SAS signature (sig=...), and any query value echoed
// in a storage service error (QueryParameterValue), with REDACTED. Scheme, host and path are kept so messages still say
// where a request went.
func RedactURLQueries(s string) string {
	s = urlQueryPattern.ReplaceAllString(s, "${1}?"+redacted)
	s = sasSignaturePattern.ReplaceAllString(s, "sig="+redacted)
	s = queryParameterValuePattern.ReplaceAllString(s, "QueryParameterValue="+redacted)
	return queryParameterValueXMLPattern.ReplaceAllString(s, "<QueryParameterValue>"+redacted+"</QueryParameterValue>")
}

// RedactURLQueriesInError returns err with RedactURLQueries applied to its message, so it can be logged or returned
// without exposing credentials carried in URLs. errors.Is still matches against the original error chain, but
// errors.As and errors.Unwrap don't reach it, so the original (which contains the credential) can't be printed by
// mistake.
func RedactURLQueriesInError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	redactedMsg := RedactURLQueries(msg)
	if redactedMsg == msg {
		return err
	}
	return &redactedError{msg: redactedMsg, err: err}
}

type redactedError struct {
	msg string
	err error
}

func (e *redactedError) Error() string { return e.msg }

func (e *redactedError) Is(target error) bool { return errors.Is(e.err, target) }
