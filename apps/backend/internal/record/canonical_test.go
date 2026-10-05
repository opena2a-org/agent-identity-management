package record

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestCanonicalJSON_OrdersMembersByUTF16CodeUnits(t *testing.T) {
	// The member names and the expected order are the sorting example of
	// RFC 8785 section 3.2.3.
	got, err := canonicalJSON(map[string]any{
		"€":          "Euro Sign",
		"\r":         "Carriage Return",
		"דּ":          "Hebrew Letter Dalet With Dagesh",
		"1":          "One",
		"\U0001f600": "Emoji: Grinning Face",
		"\u0080":     "Control",
		"ö":          "Latin Small Letter O With Diaeresis",
	})
	if err != nil {
		t.Fatalf("canonicalJSON: %v", err)
	}
	want := `{"\r":"Carriage Return","1":"One","` + "\u0080" + `":"Control","` + "ö" +
		`":"Latin Small Letter O With Diaeresis","` + "€" + `":"Euro Sign","` + "\U0001f600" +
		`":"Emoji: Grinning Face","` + "דּ" + `":"Hebrew Letter Dalet With Dagesh"}`
	if string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestCanonicalJSON_EscapesStringsAsRFC8785(t *testing.T) {
	got, err := canonicalJSON("\"\\/\b\t\n\f\r\x00\x1f\x7f<>& é")
	if err != nil {
		t.Fatalf("canonicalJSON: %v", err)
	}
	want := `"\"\\/\b\t\n\f\r\u0000\u001f` + "\x7f<>& é" + `"`
	if string(got) != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestCanonicalJSON_SerializesValuesWithoutWhitespace(t *testing.T) {
	got, err := canonicalJSON(map[string]any{
		"b": []any{true, false, nil, int64(-7), 0, "x"},
		"a": map[string]any{},
		"c": []any{},
	})
	if err != nil {
		t.Fatalf("canonicalJSON: %v", err)
	}
	if want := `{"a":{},"b":[true,false,null,-7,0,"x"],"c":[]}`; string(got) != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestCanonicalJSON_EqualValuesGiveEqualBytes(t *testing.T) {
	names := []string{"zeta", "alpha", "mid", "Alpha", "10", "9", "é", "e"}
	build := func(order []string) map[string]any {
		m := map[string]any{}
		for _, n := range order {
			m[n] = map[string]any{"name": n, "list": []any{n, int64(len(n))}}
		}
		return m
	}
	reversed := make([]string, len(names))
	for i, n := range names {
		reversed[len(names)-1-i] = n
	}
	first, err := canonicalJSON(build(names))
	if err != nil {
		t.Fatalf("canonicalJSON: %v", err)
	}
	for i := 0; i < 50; i++ {
		again, err := canonicalJSON(build(reversed))
		if err != nil {
			t.Fatalf("canonicalJSON: %v", err)
		}
		if string(again) != string(first) {
			t.Fatalf("run %d gave different bytes:\n%s\n%s", i, again, first)
		}
	}
}

func TestCanonicalJSON_RefusesValuesOutsideTheDataModel(t *testing.T) {
	cases := map[string]any{
		"float64":            1.5,
		"whole float64":      float64(2),
		"float32":            float32(1),
		"NaN":                math.NaN(),
		"integer above 2^53": int64(1) << 53,
		"integer below":      -(int64(1) << 53),
		"uint":               uint(1),
		"byte slice":         []byte("x"),
		"typed map":          map[string]string{"a": "b"},
		"time":               time.Unix(0, 0),
		"invalid UTF-8":      "a\xffb",
		"invalid UTF-8 name": map[string]any{"a\xff": 1},
		"nested float":       map[string]any{"a": []any{map[string]any{"b": 0.1}}},
	}
	for name, v := range cases {
		if _, err := canonicalJSON(v); !errors.Is(err, ErrNotCanonicalizable) {
			t.Errorf("%s: got %v; want ErrNotCanonicalizable", name, err)
		}
	}
	for name, v := range map[string]any{"largest": int64(1)<<53 - 1, "smallest": -(int64(1)<<53 - 1)} {
		if _, err := canonicalJSON(v); err != nil {
			t.Errorf("%s 53-bit integer refused: %v", name, err)
		}
	}
}

func TestFormatTimestamp_IsUTCTruncatedToMicroseconds(t *testing.T) {
	in := time.Date(2026, time.March, 4, 1, 2, 3, 999999999, time.FixedZone("", -7*60*60))
	got, err := FormatTimestamp(in)
	if err != nil {
		t.Fatalf("FormatTimestamp: %v", err)
	}
	if want := "2026-03-04T08:02:03.999999Z"; got != want {
		t.Fatalf("got %s; want %s", got, want)
	}
	whole, err := FormatTimestamp(time.Date(2026, time.March, 4, 8, 2, 3, 0, time.UTC))
	if err != nil {
		t.Fatalf("FormatTimestamp: %v", err)
	}
	if want := "2026-03-04T08:02:03.000000Z"; whole != want {
		t.Fatalf("got %s; want %s", whole, want)
	}
	if _, err := FormatTimestamp(time.Time{}); !errors.Is(err, ErrNotCanonicalizable) {
		t.Fatalf("zero time: got %v; want ErrNotCanonicalizable", err)
	}
}
