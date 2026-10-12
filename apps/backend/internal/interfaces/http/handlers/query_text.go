package handlers

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// queryTextError returns an error that names param when value is not valid
// UTF-8 or holds a NUL character, and nil otherwise. PostgreSQL refuses such a
// value as a text parameter, so a handler that compares a query value with a
// text column answers this error 400 before any statement runs.
func queryTextError(param, value string) error {
	if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
		return fmt.Errorf("%s must be UTF-8 text without a NUL character", param)
	}
	return nil
}
