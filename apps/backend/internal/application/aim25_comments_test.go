package application

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// AIM-25 — the trust calculator's comments say what factors 1, 2, 5 and 7
// compute.
//
// Four factor comments in trust_calculator.go described a design that was
// never wired ("Ed25519 signature verification for all actions", "Health check
// responsiveness", "SOC 2, HIPAA, GDPR adherence", "Behavioral pattern
// changes") rather than what the code beneath them reads. These are
// source-level assertions, in the same spirit as aim02_scope_test.go: each
// phrase below is the one the architecture decision of 2026-09-30 chose, and
// it must sit verbatim on a single `//` line in both places a reader meets the
// factor — the per-factor comment block in calculateFactorsDetailed and the
// doc comment of the factor's own function.

const aim25TrustCalculatorPath = "apps/backend/internal/application/trust_calculator.go"

// aim25Factor binds one trust factor to the comment it must carry and to the
// two anchors the comment must sit directly above.
type aim25Factor struct {
	name       string
	phrase     string
	assignment *regexp.Regexp // the factor's assignment inside calculateFactorsDetailed
	function   string         // the factor's own function
}

var aim25Factors = []aim25Factor{
	{
		name:       "factor 1",
		phrase:     "verification-event success rate over 30 days times an agent-status modifier",
		assignment: regexp.MustCompile(`^\s*factors\.VerificationStatus\s*=`),
		function:   "calculateVerificationStatus",
	},
	{
		name:       "factor 2",
		phrase:     "the same success rate adjusted for recency, used as an availability proxy",
		assignment: regexp.MustCompile(`^\s*factors\.Uptime\s*=`),
		function:   "calculateUptime",
	},
	{
		name:       "factor 5",
		phrase:     "the organization's latest AIM compliance snapshot score",
		assignment: regexp.MustCompile(`^\s*factors\.Compliance\s*,\s*reason\s*=`),
		function:   "calculateCompliance",
	},
	{
		name:       "factor 7",
		phrase:     "configuration-drift and MCP-drift alerts on the agent and its connected servers",
		assignment: regexp.MustCompile(`^\s*factors\.DriftDetection\s*,\s*reason\s*=`),
		function:   "calculateDriftDetection",
	},
}

// aim25Forbidden are the misdescriptions AC2 bans from every `//` comment line
// of the file, matched case-insensitively.
var aim25Forbidden = []string{
	"health check",
	"SOC 2",
	"HIPAA",
	"GDPR",
	"Behavioral pattern changes",
	"behavior patterns",
	"Ed25519 signature verification for all actions",
}

// aim25IsCommentLine reports whether a source line is a `//` line comment.
func aim25IsCommentLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "//")
}

// aim25BareDecl matches a bare local declaration such as `var reason string`.
// calculateFactorsDetailed declares its shared `reason` variable between the
// factor 5 comment block and the factor 5 assignment; a declaration with no
// initializer carries no factor data, so the block above it still counts as
// the block directly above the assignment. This keeps the delivery's diff to
// comment text only, as AIM-25.AC4 requires.
var aim25BareDecl = regexp.MustCompile(`^\s*var\s+\w+\s+\w+\s*$`)

// aim25CommentBlockAbove returns the contiguous run of `//` comment lines that
// sits directly above line index i (the line at i itself is excluded), looking
// past a single bare declaration line if one sits in between.
func aim25CommentBlockAbove(lines []string, i int) []string {
	j := i - 1
	if j >= 0 && aim25BareDecl.MatchString(lines[j]) {
		j--
	}
	var block []string
	for ; j >= 0 && aim25IsCommentLine(lines[j]); j-- {
		block = append([]string{lines[j]}, block...)
	}
	return block
}

// aim25FindLine returns the index of the first line matching re within
// [from, to), or -1.
func aim25FindLine(lines []string, re *regexp.Regexp, from, to int) int {
	for i := from; i < to && i < len(lines); i++ {
		if re.MatchString(lines[i]) {
			return i
		}
	}
	return -1
}

// aim25FuncBounds returns the line index of the given method's `func` line and
// the index of the next top-level `func` after it (or len(lines)).
func aim25FuncBounds(t *testing.T, lines []string, method string) (int, int) {
	t.Helper()
	start := aim25FindLine(lines, regexp.MustCompile(`^func \(c \*TrustCalculator\) `+regexp.QuoteMeta(method)+`\(`), 0, len(lines))
	require.NotEqual(t, -1, start, "trust_calculator.go must define %s", method)
	end := aim25FindLine(lines, regexp.MustCompile(`^func `), start+1, len(lines))
	if end == -1 {
		end = len(lines)
	}
	return start, end
}

// aim25BlockHasPhrase asserts that one single line of block contains phrase
// verbatim.
func aim25BlockHasPhrase(t *testing.T, block []string, phrase, where string) {
	t.Helper()
	for _, line := range block {
		if aim25IsCommentLine(line) && strings.Contains(line, phrase) {
			return
		}
	}
	t.Errorf("no single `//` comment line %s carries the phrase %q; the comment block there is:\n%s",
		where, phrase, strings.Join(block, "\n"))
}

func TestAIM25_TrustCalculatorCommentsSayWhatFactorsCompute(t *testing.T) {
	root := repoRoot(t)
	src := aim02ReadFile(t, root, aim25TrustCalculatorPath)
	lines := strings.Split(src, "\n")

	detailedStart, detailedEnd := aim25FuncBounds(t, lines, "calculateFactorsDetailed")

	for _, f := range aim25Factors {
		f := f
		t.Run("AIM-25.AC1 "+f.name+" comment above its assignment in calculateFactorsDetailed", func(t *testing.T) {
			i := aim25FindLine(lines, f.assignment, detailedStart, detailedEnd)
			require.NotEqual(t, -1, i, "calculateFactorsDetailed must assign %s (%s)", f.name, f.assignment)
			block := aim25CommentBlockAbove(lines, i)
			require.NotEmpty(t, block, "the assignment for %s must have comment lines directly above it", f.name)
			aim25BlockHasPhrase(t, block, f.phrase, "directly above the "+f.name+" assignment in calculateFactorsDetailed")
		})

		t.Run("AIM-25.AC1 "+f.name+" doc comment on "+f.function, func(t *testing.T) {
			i, _ := aim25FuncBounds(t, lines, f.function)
			block := aim25CommentBlockAbove(lines, i)
			require.NotEmpty(t, block, "%s must have a doc comment directly above it", f.function)
			aim25BlockHasPhrase(t, block, f.phrase, "in the doc comment of "+f.function)
		})
	}

	for _, banned := range aim25Forbidden {
		banned := banned
		t.Run("AIM-25.AC2 no comment line says "+banned, func(t *testing.T) {
			needle := strings.ToLower(banned)
			for n, line := range lines {
				if !aim25IsCommentLine(line) {
					continue
				}
				assert.NotContains(t, strings.ToLower(line), needle,
					"trust_calculator.go:%d still describes a factor by a data source the code does not read", n+1)
			}
		})
	}

	t.Run("AIM-25.AC3 the file still does not touch signing-key handling", func(t *testing.T) {
		// The same invariant aim02_scope_test.go holds over this file; restated
		// here so the comment rewrite is seen to have left it alone.
		lower := strings.ToLower(src)
		assert.NotContains(t, lower, "keyvault")
		assert.NotContains(t, lower, "master_key")
	})
}
