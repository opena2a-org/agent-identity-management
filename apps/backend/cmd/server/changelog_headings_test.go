package main

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// changelogTypedHeading matches a third-level CHANGELOG.md heading that opens
// with a Keep a Changelog change type, and captures what follows the type.
var changelogTypedHeading = regexp.MustCompile(`^### (Added|Changed|Deprecated|Removed|Fixed|Security)(.*)$`)

// changelogHeadingSeparators are the ways a typed heading may separate the
// change type from its summary: a colon (`### Fixed: summary`), the form to
// write, and the em dash (`### Fixed — summary`) the earlier entries carry.
var changelogHeadingSeparators = []string{": ", " — "}

// changelogHeadingFindings returns one line per typed heading that neither
// stands alone (`### Fixed`) nor separates the type from a summary with one of
// changelogHeadingSeparators.
func changelogHeadingFindings(changelog string) []string {
	var findings []string
	for i, line := range strings.Split(changelog, "\n") {
		m := changelogTypedHeading.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		rest := m[2]
		if rest == "" {
			continue
		}
		separated := false
		for _, sep := range changelogHeadingSeparators {
			if summary, ok := strings.CutPrefix(rest, sep); ok && strings.TrimSpace(summary) != "" {
				separated = true
				break
			}
		}
		if separated {
			continue
		}
		findings = append(findings, fmt.Sprintf("CHANGELOG.md:%d: %s", i+1, line))
	}
	return findings
}

func TestChangelogHeadingFindings(t *testing.T) {
	sample := strings.Join([]string{
		"## [Unreleased]",
		"",
		"### Fixed: the guide's deployment step starts the server",
		"### Fixed — the token endpoint reports the lifetime of the token it issues",
		"### Removed - the second Dockerfile",
		"### Security —",
		"### Security:",
		"### Changed:the dashboard lists MCP servers",
		"### Fixed",
		"### Notes",
	}, "\n")

	assert.Equal(t, []string{
		"CHANGELOG.md:5: ### Removed - the second Dockerfile",
		"CHANGELOG.md:6: ### Security —",
		"CHANGELOG.md:7: ### Security:",
		"CHANGELOG.md:8: ### Changed:the dashboard lists MCP servers",
	}, changelogHeadingFindings(sample))
}

// Every typed heading in CHANGELOG.md separates the change type from its
// summary with a colon, or with the em dash the earlier entries carry; no
// heading runs its summary into the type or leaves it empty.
func TestChangelogTypedHeadingsSeparateTypeFromSummary(t *testing.T) {
	requirePublicRepositoryFiles(t)

	findings := changelogHeadingFindings(aim03ReadRepoFile(t, "CHANGELOG.md"))
	assert.Empty(t, findings, "write each heading as `### <Type>: <summary>`:\n%s", strings.Join(findings, "\n"))
}
