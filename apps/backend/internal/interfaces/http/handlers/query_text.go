package handlers

import (
	"strings"
	"unicode/utf8"
)

// queryTextMessage is the line a handler answers, with 400, for a query
// parameter whose value is not UTF-8 text without a NUL character.
func queryTextMessage(param string) string {
	return param + " must be UTF-8 text without a NUL character"
}

// notTextError is the error queryTextError returns. It names the parameter, so
// a handler that receives it from a helper answers queryTextMessage for that
// parameter instead of copying the error's text into the response.
type notTextError struct{ param string }

func (e notTextError) Error() string { return queryTextMessage(e.param) }

// queryTextError returns an error that names param when value is not valid
// UTF-8 or holds a NUL character, and nil otherwise. PostgreSQL refuses such a
// value as a text parameter, so a handler that compares a query value with a
// text column answers queryTextMessage with 400 before any statement runs.
func queryTextError(param, value string) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return notTextError{param: param}
	}
	return nil
}
