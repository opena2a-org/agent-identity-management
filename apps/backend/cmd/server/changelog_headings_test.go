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

// changelogHeadingFindings returns one line per typed heading that neither
// stands alone (`### Fixed`) nor separates the type from its summary with an
// em dash (`### Fixed — summary`).
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
		if summary, ok := strings.CutPrefix(rest, " — "); ok && strings.TrimSpace(summary) != "" {
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
		"### Fixed — the token endpoint reports the lifetime of the token it issues",
		"### Fixed: the guide's deployment step starts the server",
		"### Removed - the second Dockerfile",
		"### Security —",
		"### Fixed",
		"### Notes",
	}, "\n")

	assert.Equal(t, []string{
		"CHANGELOG.md:4: ### Fixed: the guide's deployment step starts the server",
		"CHANGELOG.md:5: ### Removed - the second Dockerfile",
		"CHANGELOG.md:6: ### Security —",
	}, changelogHeadingFindings(sample))
}

// Every typed heading in CHANGELOG.md separates the change type from its
// summary the same way, so the entries read as one list.
func TestChangelogTypedHeadingsUseEmDash(t *testing.T) {
	findings := changelogHeadingFindings(aim03ReadRepoFile(t, "CHANGELOG.md"))
	assert.Empty(t, findings, "write each heading as `### <Type> — <summary>`:\n%s", strings.Join(findings, "\n"))
}
