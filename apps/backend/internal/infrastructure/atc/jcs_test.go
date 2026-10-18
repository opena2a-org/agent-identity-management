package atc

import (
	"errors"
	"strings"
	"testing"
)

// canonicalJSON returns the RFC 8785 form of the JSON document data, read with
// the reader the revocation list client uses.
func canonicalJSON(data []byte, limit int) ([]byte, error) {
	r, err := newJSONReader(data)
	if err != nil {
		return nil, err
	}
	out, _, err := r.next(nil, 0, limit)
	if err != nil {
		return nil, err
	}
	if err := r.end(); err != nil {
		return nil, err
	}
	return out, nil
}

func mustCanonicalJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	out, err := canonicalJSON(data, len(data)+1024)
	if err != nil {
		t.Fatalf("canonical form of %.80q: %v", data, err)
	}
	return out
}

// The example of RFC 8785 section 3.2.2, sorted and written without white
// space.
func TestCanonicalJSON_WritesTheRFC8785Example(t *testing.T) {
	input := `{
  "numbers": [333333333.33333329, 1E30, 4.50, 2e-3, 0.000000000000000000000000001],
  "string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/",
  "literals": [null, true, false]
}`
	want := `{"literals":[null,true,false],"numbers":[333333333.3333333,1e+30,4.5,0.002,1e-27],"string":"€$\u000f\nA'B\"\\\\\"/"}`
	if got := string(mustCanonicalJSON(t, []byte(input))); got != want {
		t.Fatalf("canonical form\n got %s\nwant %s", got, want)
	}
}

// The sorting example of RFC 8785 section 3.2.3: member names are ordered by
// UTF-16 code units, so the emoji, a surrogate pair, sorts before U+FB33.
func TestCanonicalJSON_OrdersMembersByUTF16CodeUnits(t *testing.T) {
	input := `{
  "\u20ac": "Euro Sign",
  "\r": "Carriage Return",
  "\ufb33": "Hebrew Letter Dalet With Dagesh",
  "1": "One",
  "\ud83d\ude00": "Emoji: Grinning Face",
  "\u0080": "Control",
  "\u00f6": "Latin Small Letter O With Diaeresis"
}`
	want := "{\"\\r\":\"Carriage Return\",\"1\":\"One\",\"\u0080\":\"Control\"," +
		"\"\u00f6\":\"Latin Small Letter O With Diaeresis\",\"\u20ac\":\"Euro Sign\"," +
		"\"\U0001F600\":\"Emoji: Grinning Face\",\"\ufb33\":\"Hebrew Letter Dalet With Dagesh\"}"
	if got := string(mustCanonicalJSON(t, []byte(input))); got != want {
		t.Fatalf("canonical form\n got %q\nwant %q", got, want)
	}
}

// A number is written in the ECMAScript form of its nearest binary64 value.
func TestCanonicalJSON_WritesNumbersInTheirECMAScriptForm(t *testing.T) {
	cases := map[string]string{
		"0":                       "0",
		"-0":                      "0",
		"1":                       "1",
		"-1.5":                    "-1.5",
		"4.50":                    "4.5",
		"2e-3":                    "0.002",
		"0.000001":                "0.000001",
		"0.0000001":               "1e-7",
		"1e-27":                   "1e-27",
		"100000000000000000000":   "100000000000000000000",
		"1e21":                    "1e+21",
		"1E30":                    "1e+30",
		"333333333.33333329":      "333333333.3333333",
		"9007199254740993":        "9007199254740992",
		"9223372036854775807":     "9223372036854776000",
		"5e-324":                  "5e-324",
		"1.7976931348623157e308":  "1.7976931348623157e+308",
		"-1.7976931348623157e308": "-1.7976931348623157e+308",
	}
	for input, want := range cases {
		if got := string(mustCanonicalJSON(t, []byte(input))); got != want {
			t.Errorf("canonical form of %s is %s, want %s", input, got, want)
		}
	}
}

// What has no single canonical form, or is out of bounds, is refused.
func TestCanonicalJSON_RefusesADocumentWithoutOneCanonicalForm(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"a member named twice", `{"a":1,"b":2,"a":1}`},
		{"a member named twice in a nested object", `{"a":{"b":1,"b":1}}`},
		{"invalid UTF-8", "{\"a\":\"\xff\"}"},
		{"a number beyond binary64", `[1e400]`},
		{"data after the value", `{"a":1} {}`},
		{"arrays nested 17 deep", strings.Repeat("[", 17) + strings.Repeat("]", 17)},
		{"no value", ``},
	}
	for _, tc := range cases {
		if out, err := canonicalJSON([]byte(tc.input), 1<<20); err == nil {
			t.Errorf("%s: canonical form %q, want a refusal", tc.name, out)
		}
	}
	if _, err := canonicalJSON([]byte(strings.Repeat("[", 16)+strings.Repeat("]", 16)), 1<<20); err != nil {
		t.Errorf("arrays nested 16 deep: %v", err)
	}
}

// A value whose canonical form is longer than its bound is refused while it
// is read, and one at the bound is not.
func TestCanonicalJSON_RefusesAValueLongerThanItsBound(t *testing.T) {
	doc := []byte(`{"b":[1,2,3],"a":"x"}`) // canonical form {"a":"x","b":[1,2,3]}, 21 bytes
	if _, err := canonicalJSON(doc, 21); err != nil {
		t.Fatalf("at the bound: %v", err)
	}
	if _, err := canonicalJSON(doc, 20); !errors.Is(err, errCanonicalLimit) {
		t.Fatalf("one byte over the bound: %v, want errCanonicalLimit", err)
	}
}
