package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestScanFile(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		expect []string
	}{
		{
			name: "bad: top-level var slice in handler-shaped function",
			path: "testdata/bad_handler.go",
			expect: []string{
				"history []*TrustScoreEntry",
				"alerts []*Alert",
				"names []string",
			},
		},
		{
			name:   "good: make([]T, 0) initialization",
			path:   "testdata/good_handler.go",
			expect: nil,
		},
		{
			name:   "exempt: []byte, []interface{}, []map[K]V scratch targets",
			path:   "testdata/exempt_scratch.go",
			expect: nil,
		},
		{
			name: "nested: var slice inside if/for/range/switch",
			path: "testdata/nested_blocks.go",
			expect: []string{
				"a []string",
				"b []int",
				"c []float64",
				"d []bool",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings, err := scanFile(tc.path)
			if err != nil {
				t.Fatalf("scanFile(%s): %v", tc.path, err)
			}
			got := make([]string, 0, len(findings))
			for _, f := range findings {
				got = append(got, f.Name+" []"+f.Type)
			}
			sort.Strings(got)

			want := append([]string(nil), tc.expect...)
			sort.Strings(want)

			if len(got) != len(want) {
				t.Fatalf("finding count: got %d (%v), want %d (%v)", len(got), got, len(want), want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("finding[%d]: got %q, want %q", i, got[i], want[i])
				}
			}
		})
	}
}

func TestLoadBaseline(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "baseline-*.txt")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	const content = `# comment
internal/foo.go:42
internal/bar.go:7

# blank above
internal/baz.go:99
`
	if _, err := tmp.WriteString(content); err != nil {
		t.Fatalf("write temp: %v", err)
	}
	tmp.Close()

	baseline, err := loadBaseline(tmp.Name())
	if err != nil {
		t.Fatalf("loadBaseline: %v", err)
	}
	want := []string{"internal/foo.go:42", "internal/bar.go:7", "internal/baz.go:99"}
	for _, w := range want {
		if _, ok := baseline[w]; !ok {
			t.Errorf("baseline missing %q", w)
		}
	}
	if len(baseline) != len(want) {
		t.Errorf("baseline size: got %d, want %d", len(baseline), len(want))
	}
}

func TestLoadBaselineMissing(t *testing.T) {
	baseline, err := loadBaseline(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("loadBaseline (missing): %v", err)
	}
	if len(baseline) != 0 {
		t.Errorf("missing-baseline size: got %d, want 0", len(baseline))
	}
}

func TestScanFileMissingFile(t *testing.T) {
	_, err := scanFile(filepath.Join(t.TempDir(), "does-not-exist.go"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}
