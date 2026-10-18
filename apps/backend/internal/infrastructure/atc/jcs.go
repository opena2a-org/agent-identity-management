package atc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// maxJSONDepth bounds how deeply a document the revocation list client reads
// may nest arrays and objects.
const maxJSONDepth = 16

// errCanonicalLimit is returned when the canonical form of a value outgrows
// the bytes it is allowed.
var errCanonicalLimit = errors.New("the canonical form exceeds its size bound")

// jsonReader reads the values of one JSON document and writes the RFC 8785
// (JSON Canonicalization Scheme) form of each value it reads. A value is
// returned as nil, bool, string, json.Number, []any or map[string]any; a
// json.Number keeps the digits the document wrote.
type jsonReader struct {
	dec *json.Decoder
}

// newJSONReader returns a reader of data, which must be valid UTF-8.
func newJSONReader(data []byte) (*jsonReader, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("the document is not valid UTF-8")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return &jsonReader{dec: dec}, nil
}

// next reads the next value, appends its canonical form to out, and returns
// out and the value. depth is the number of arrays and objects that enclose
// the value. It refuses an array or object enclosed by maxJSONDepth others, an
// object that names a member twice, a number beyond the binary64 range, and
// an out longer than limit bytes, which it checks as the value is read.
func (r *jsonReader) next(out []byte, depth, limit int) ([]byte, any, error) {
	tok, err := r.dec.Token()
	if err != nil {
		return nil, nil, err
	}
	var value any
	switch t := tok.(type) {
	case json.Delim:
		if depth >= maxJSONDepth {
			return nil, nil, fmt.Errorf("the document nests arrays and objects deeper than %d levels", maxJSONDepth)
		}
		switch t {
		case '[':
			out, value, err = r.array(out, depth, limit)
		case '{':
			out, value, err = r.object(out, depth, limit)
		default:
			err = fmt.Errorf("unexpected %q", rune(t))
		}
		if err != nil {
			return nil, nil, err
		}
	case string:
		out = appendCanonicalString(out, t)
		value = t
	case json.Number:
		s, err := canonicalNumber(t)
		if err != nil {
			return nil, nil, err
		}
		out = append(out, s...)
		value = t
	case bool:
		out = strconv.AppendBool(out, t)
		value = t
	case nil:
		out = append(out, "null"...)
	}
	if len(out) > limit {
		return nil, nil, errCanonicalLimit
	}
	return out, value, nil
}

func (r *jsonReader) array(out []byte, depth, limit int) ([]byte, any, error) {
	out = append(out, '[')
	values := []any{}
	for r.dec.More() {
		if len(values) > 0 {
			out = append(out, ',')
		}
		var value any
		var err error
		if out, value, err = r.next(out, depth+1, limit); err != nil {
			return nil, nil, err
		}
		values = append(values, value)
	}
	if _, err := r.dec.Token(); err != nil {
		return nil, nil, err
	}
	return append(out, ']'), values, nil
}

func (r *jsonReader) object(out []byte, depth, limit int) ([]byte, any, error) {
	type member struct {
		name      string
		canonical []byte // "name":value
	}
	values := map[string]any{}
	var members []member
	// size is the length out has once the members read so far are written.
	size := len(out) + len("{}")
	for r.dec.More() {
		tok, err := r.dec.Token()
		if err != nil {
			return nil, nil, err
		}
		name, _ := tok.(string)
		if _, named := values[name]; named {
			return nil, nil, fmt.Errorf("an object names member %q twice", name)
		}
		if len(members) > 0 {
			size++ // the comma before this member
		}
		// The member may use the bytes the bound leaves.
		canonical, value, err := r.next(append(appendCanonicalString(nil, name), ':'), depth+1, limit-size)
		if err != nil {
			return nil, nil, err
		}
		size += len(canonical)
		values[name] = value
		members = append(members, member{name: name, canonical: canonical})
	}
	if _, err := r.dec.Token(); err != nil {
		return nil, nil, err
	}
	sort.Slice(members, func(i, j int) bool { return lessUTF16(members[i].name, members[j].name) })
	out = append(out, '{')
	for i, m := range members {
		if i > 0 {
			out = append(out, ',')
		}
		out = append(out, m.canonical...)
	}
	return append(out, '}'), values, nil
}

// end refuses anything but white space after the document's value.
func (r *jsonReader) end() error {
	if _, err := r.dec.Token(); err != io.EOF {
		return errors.New("the document carries data after its value")
	}
	return nil
}

// canonicalNumber writes n as RFC 8785 section 3.2.2.3 requires: the
// ECMAScript form of the nearest binary64 value, which is plain decimal
// notation from 1e-6 up to 1e21 and exponent notation outside it.
func canonicalNumber(n json.Number) (string, error) {
	f, err := strconv.ParseFloat(string(n), 64)
	if err != nil {
		return "", fmt.Errorf("the number %s is beyond the binary64 range", n)
	}
	if f == 0 {
		return "0", nil
	}
	if abs := math.Abs(f); abs >= 1e-6 && abs < 1e21 {
		return strconv.FormatFloat(f, 'f', -1, 64), nil
	}
	// strconv writes at least two exponent digits ("1e-07"); ECMAScript
	// writes the digits the exponent needs ("1e-7").
	mantissa, exponent, _ := strings.Cut(strconv.FormatFloat(f, 'e', -1, 64), "e")
	return mantissa + "e" + exponent[:1] + strings.TrimLeft(exponent[1:], "0"), nil
}

// appendCanonicalString appends s as RFC 8785 section 3.2.2.2 requires: the
// two mandatory escapes, the short escapes for the five named control
// characters, lowercase \u00xx for the other control characters, and the
// other characters as their UTF-8 bytes.
func appendCanonicalString(out []byte, s string) []byte {
	const hex = "0123456789abcdef"
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			out = append(out, '\\', '"')
		case c == '\\':
			out = append(out, '\\', '\\')
		case c >= 0x20:
			out = append(out, c)
		case c == '\b':
			out = append(out, '\\', 'b')
		case c == '\t':
			out = append(out, '\\', 't')
		case c == '\n':
			out = append(out, '\\', 'n')
		case c == '\f':
			out = append(out, '\\', 'f')
		case c == '\r':
			out = append(out, '\\', 'r')
		default:
			out = append(out, '\\', 'u', '0', '0', hex[c>>4], hex[c&0x0f])
		}
	}
	return append(out, '"')
}

// lessUTF16 orders member names by their UTF-16 code units, the order RFC 8785
// section 3.2.3 requires. It differs from byte order for characters outside
// the Basic Multilingual Plane.
func lessUTF16(a, b string) bool {
	ua, ub := utf16.Encode([]rune(a)), utf16.Encode([]rune(b))
	for i := 0; i < len(ua) && i < len(ub); i++ {
		if ua[i] != ub[i] {
			return ua[i] < ub[i]
		}
	}
	return len(ua) < len(ub)
}
