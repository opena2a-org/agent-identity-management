package record

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// maxSafeInteger is the largest integer the restricted data model admits
// (2^53 - 1): every JSON implementation reads it back exactly.
const maxSafeInteger = 1<<53 - 1

// timestampLayout renders a timestamp in RFC 3339, UTC, with exactly six
// fractional digits, so equal instants give equal bytes.
const timestampLayout = "2006-01-02T15:04:05.000000Z"

// ErrNotCanonicalizable is wrapped by every error the serializer returns for a
// value outside the restricted data model.
var ErrNotCanonicalizable = errors.New("record: value is outside the canonical data model")

// FormatTimestamp renders t as the canonical form of a timestamp: RFC 3339,
// UTC, truncated to microseconds. The zero time is refused.
func FormatTimestamp(t time.Time) (string, error) {
	if t.IsZero() {
		return "", fmt.Errorf("%w: timestamp is not set", ErrNotCanonicalizable)
	}
	u := t.UTC().Truncate(time.Microsecond)
	if y := u.Year(); y < 1 || y > 9999 {
		return "", fmt.Errorf("%w: timestamp year %d is outside RFC 3339", ErrNotCanonicalizable, y)
	}
	return u.Format(timestampLayout), nil
}

// canonicalJSON serializes v per RFC 8785 over the restricted data model.
// Admitted values: nil, bool, string (valid UTF-8), int and int64 within 53
// bits, []any and map[string]any of admitted values. Anything else, a floating
// point number included, is refused.
func canonicalJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v, "$"); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v any, path string) error {
	switch x := v.(type) {
	case nil:
		buf.WriteString("null")
	case bool:
		if x {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
	case string:
		return writeCanonicalString(buf, x, path)
	case int:
		return writeCanonicalInteger(buf, int64(x), path)
	case int64:
		return writeCanonicalInteger(buf, x, path)
	case []any:
		buf.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonical(buf, item, path+"["+strconv.Itoa(i)+"]"); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			if !utf8.ValidString(k) {
				return fmt.Errorf("%w: a member name under %s is not valid UTF-8", ErrNotCanonicalizable, path)
			}
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return lessUTF16(keys[i], keys[j]) })
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeCanonicalString(buf, k, path); err != nil {
				return err
			}
			buf.WriteByte(':')
			if err := writeCanonical(buf, x[k], path+"."+k); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		return fmt.Errorf("%w: %s holds a %T", ErrNotCanonicalizable, path, v)
	}
	return nil
}

func writeCanonicalInteger(buf *bytes.Buffer, n int64, path string) error {
	if n > maxSafeInteger || n < -maxSafeInteger {
		return fmt.Errorf("%w: %s holds an integer beyond 53 bits", ErrNotCanonicalizable, path)
	}
	buf.WriteString(strconv.FormatInt(n, 10))
	return nil
}

// writeCanonicalString writes s as RFC 8785 section 3.2.2.2 requires: the two
// mandatory escapes, the short escapes for the five named control characters,
// lowercase \u00xx for the other control characters, and every other character
// as its UTF-8 bytes.
func writeCanonicalString(buf *bytes.Buffer, s string, path string) error {
	if !utf8.ValidString(s) {
		return fmt.Errorf("%w: a string at %s is not valid UTF-8", ErrNotCanonicalizable, path)
	}
	const hex = "0123456789abcdef"
	buf.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			buf.WriteString(`\"`)
		case c == '\\':
			buf.WriteString(`\\`)
		case c == '\b':
			buf.WriteString(`\b`)
		case c == '\t':
			buf.WriteString(`\t`)
		case c == '\n':
			buf.WriteString(`\n`)
		case c == '\f':
			buf.WriteString(`\f`)
		case c == '\r':
			buf.WriteString(`\r`)
		case c < 0x20:
			buf.WriteString(`\u00`)
			buf.WriteByte(hex[c>>4])
			buf.WriteByte(hex[c&0x0f])
		default:
			buf.WriteByte(c)
		}
	}
	buf.WriteByte('"')
	return nil
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
