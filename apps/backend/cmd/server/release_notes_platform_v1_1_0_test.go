package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The platform 1.1.0 release notes cite each security fix by its CHANGELOG.md
// heading and name the migrations an upgrade from 1.0.0 applies. The tests in
// this file hold both to the tree, so a Security entry or a migration that
// lands before the tag is a finding here instead of a gap in the notes.

const (
	platform110ReleaseNotes = "docs/releases/platform-v1.1.0.md"

	// platform100LastMigration is the last of the migration files the
	// platform-v1.0.0 tag carries, and platform100Migrations how many it
	// carries:
	//   git ls-tree -r --name-only platform-v1.0.0 apps/backend/migrations | grep -c '\.sql$'
	// A 1.0.0 database records each of them in schema_migrations, so the
	// upgrade applies every migration file the runner sorts after the last.
	platform100LastMigration = "097_backfill_a2a_consent_organization_id.sql"
	platform100Migrations    = 97
)

var (
	// changelogReleaseHeading matches a second-level CHANGELOG.md heading and
	// captures the release it opens: "Unreleased" or a version.
	changelogReleaseHeading = regexp.MustCompile(`^## \[([^\]]+)\]`)
	// changelogSecurityHeading captures the summary of a typed Security
	// heading, written with either separator changelogHeadingSeparators allows.
	changelogSecurityHeading = regexp.MustCompile(`^### Security(?:: | — )(.+)$`)
	// releaseNotesSecurityRow captures the heading a row of the notes' security
	// table cites: `| Security: <summary> | <fixed in> |`.
	releaseNotesSecurityRow = regexp.MustCompile(`^\| Security: (.+?) \|`)
	// releaseNotesTimedRunLog captures the count in the line runMigrations logs
	// when it applies migrations, as the notes quote it for the timed runs of
	// `aim-server migrate`: "Successfully applied <n> pending migration(s)".
	releaseNotesTimedRunLog = regexp.MustCompile(`Successfully applied (\d+) pending migration\(s\)`)
)

// changelogReleaseSection returns the lines of CHANGELOG.md between the
// `## [release]` heading and the next second-level release heading, and
// whether the changelog has that heading.
func changelogReleaseSection(changelog, release string) (string, bool) {
	var section []string
	found, in := false, false
	for _, line := range strings.Split(changelog, "\n") {
		line = strings.TrimRight(line, "\r")
		if m := changelogReleaseHeading.FindStringSubmatch(line); m != nil {
			in = m[1] == release
			found = found || in
			continue
		}
		if in {
			section = append(section, line)
		}
	}
	return strings.Join(section, "\n"), found
}

// platform110ChangelogSection returns the CHANGELOG.md section platform 1.1.0
// is written from: `[1.1.0]` once the changelog has it, `[Unreleased]` until
// then. released reports which.
func platform110ChangelogSection(changelog string) (section string, released bool) {
	if section, ok := changelogReleaseSection(changelog, "1.1.0"); ok {
		return section, true
	}
	section, _ = changelogReleaseSection(changelog, "Unreleased")
	return section, false
}

// changelogSecurityHeadings returns the summary of each typed Security heading
// in a CHANGELOG.md section, in order. The plain `### Security` heading, which
// heads a list of entries, has no summary and is not returned.
func changelogSecurityHeadings(section string) []string {
	var headings []string
	for _, line := range strings.Split(section, "\n") {
		if m := changelogSecurityHeading.FindStringSubmatch(strings.TrimRight(line, "\r")); m != nil {
			headings = append(headings, strings.TrimSpace(m[1]))
		}
	}
	return headings
}

// releaseNotesSecurityRowFindings returns one line per row of the notes'
// security table that cites no Security heading of the section, and one per
// Security heading of the section that no row cites.
func releaseNotesSecurityRowFindings(section, notes string) []string {
	headings := map[string]bool{}
	for _, h := range changelogSecurityHeadings(section) {
		headings[h] = true
	}
	cited := map[string]bool{}
	var findings []string
	for i, line := range strings.Split(notes, "\n") {
		m := releaseNotesSecurityRow.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		cited[m[1]] = true
		if !headings[m[1]] {
			findings = append(findings, fmt.Sprintf("%s:%d: the row cites `Security: %s`, which is not a Security heading of the release in CHANGELOG.md",
				platform110ReleaseNotes, i+1, m[1]))
		}
	}
	for _, h := range changelogSecurityHeadings(section) {
		if !cited[h] {
			findings = append(findings, fmt.Sprintf("CHANGELOG.md: no row cites `Security: %s`; add `| Security: %s | <fixed in> |`", h, h))
		}
	}
	return findings
}

// platform100UpgradeMigrations splits the file names of apps/backend/migrations
// into the migration files the 1.0.0 tag carries and the ones an upgrade from
// 1.0.0 applies, each in the order the runner applies them. It selects the
// files getMigrationFiles selects: a .sql name that is not a .down.sql one.
func platform100UpgradeMigrations(names []string) (carried, applied []string) {
	var files []string
	for _, name := range names {
		if strings.HasSuffix(name, ".sql") && !strings.Contains(name, ".down.sql") {
			files = append(files, name)
		}
	}
	sort.Strings(files)
	for _, file := range files {
		if file <= platform100LastMigration {
			carried = append(carried, file)
		} else {
			applied = append(applied, file)
		}
	}
	return carried, applied
}

// releaseNotesMigrationRangeFindings returns a finding unless the notes say
// how many migrations the upgrade applies and name the first and the last,
// as "applies <n> migrations, `<first>` to `<last>`", in any line wrapping.
func releaseNotesMigrationRangeFindings(notes string, applied []string) []string {
	if len(applied) == 0 {
		return []string{"no migration file sorts after " + platform100LastMigration}
	}
	want := fmt.Sprintf("applies %d migrations, `%s` to `%s`", len(applied), applied[0], applied[len(applied)-1])
	if strings.Contains(strings.Join(strings.Fields(notes), " "), want) {
		return nil
	}
	return []string{fmt.Sprintf("%s does not say: %s", platform110ReleaseNotes, want)}
}

// releaseNotesTimedRunFindings returns a finding unless the notes quote the
// line the timed runs logged, "Successfully applied <n> pending migration(s)"
// in any line wrapping, and one for each such line whose count is not the
// number of migrations the upgrade applies: a time measured before a migration
// existed is not the time of the upgrade.
func releaseNotesTimedRunFindings(notes string, applied []string) []string {
	matches := releaseNotesTimedRunLog.FindAllStringSubmatch(strings.Join(strings.Fields(notes), " "), -1)
	if len(matches) == 0 {
		return []string{fmt.Sprintf("%s quotes no line the timed runs logged: Successfully applied %d pending migration(s)",
			platform110ReleaseNotes, len(applied))}
	}
	var findings []string
	for _, m := range matches {
		if n, err := strconv.Atoi(m[1]); err != nil || n != len(applied) {
			findings = append(findings, fmt.Sprintf("%s says the timed runs logged `%s`; the upgrade applies %d migrations: time it again",
				platform110ReleaseNotes, m[0], len(applied)))
		}
	}
	return findings
}

func TestReleaseNotesSecurityRowFindings(t *testing.T) {
	changelog := strings.Join([]string{
		"# Changelog",
		"",
		"## [Unreleased]",
		"",
		"### Security — the token endpoint refuses a revoked agent",
		"",
		"### Security: the dashboard sends a Content-Security-Policy",
		"",
		"### Fixed — the guide names the route",
		"",
		"### Security",
		"",
		"- An entry of the plain Security list.",
		"",
		"## [1.0.0] - 2026-06-01",
		"",
		"### Security — a fix that shipped in 1.0.0",
	}, "\n")
	notes := strings.Join([]string{
		"| `CHANGELOG.md` heading | Fixed in |",
		"|------------------------|----------|",
		"| Security: the token endpoint refuses a revoked agent | 1.1.0 |",
		"| Security: a fix that shipped in 1.0.0 | 1.1.0 |",
		"| Fixed: the guide names the route | 1.1.0 |",
	}, "\n")

	section, released := platform110ChangelogSection(changelog)
	assert.False(t, released)
	assert.Equal(t, []string{
		"the token endpoint refuses a revoked agent",
		"the dashboard sends a Content-Security-Policy",
	}, changelogSecurityHeadings(section))
	assert.Equal(t, []string{
		"docs/releases/platform-v1.1.0.md:4: the row cites `Security: a fix that shipped in 1.0.0`, which is not a Security heading of the release in CHANGELOG.md",
		"CHANGELOG.md: no row cites `Security: the dashboard sends a Content-Security-Policy`; add `| Security: the dashboard sends a Content-Security-Policy | <fixed in> |`",
	}, releaseNotesSecurityRowFindings(section, notes))
}

func TestPlatform110ChangelogSectionIsTheReleasedOneOnceItExists(t *testing.T) {
	changelog := strings.Join([]string{
		"## [Unreleased]",
		"",
		"### Security — a fix that landed after the tag",
		"",
		"## [1.1.0] - 2026-10-21",
		"",
		"### Security — a fix in the tag",
		"",
		"## [1.0.0] - 2026-06-01",
		"",
		"### Security — a fix in 1.0.0",
	}, "\n")

	section, released := platform110ChangelogSection(changelog)
	assert.True(t, released)
	assert.Equal(t, []string{"a fix in the tag"}, changelogSecurityHeadings(section))
}

func TestReleaseNotesMigrationRangeFindings(t *testing.T) {
	carried, applied := platform100UpgradeMigrations([]string{
		"101_d.sql", "001_a.sql", "README.md", platform100LastMigration, "099_c.down.sql", "098_b.up.sql",
	})
	assert.Equal(t, []string{"001_a.sql", platform100LastMigration}, carried)
	assert.Equal(t, []string{"098_b.up.sql", "101_d.sql"}, applied)

	assert.Empty(t, releaseNotesMigrationRangeFindings(
		"Upgrading from 1.0.0 applies 2 migrations, `098_b.up.sql` to\n`101_d.sql`. Measured on a laptop.", applied))
	assert.Equal(t, []string{
		"docs/releases/platform-v1.1.0.md does not say: applies 2 migrations, `098_b.up.sql` to `101_d.sql`",
	}, releaseNotesMigrationRangeFindings(
		"Upgrading from 1.0.0 applies 3 migrations, `098_b.up.sql` to `102_e.sql`.", applied))
	assert.Equal(t, []string{"no migration file sorts after " + platform100LastMigration},
		releaseNotesMigrationRangeFindings("applies 0 migrations", nil))
}

func TestReleaseNotesTimedRunFindings(t *testing.T) {
	applied := []string{"098_b.up.sql", "101_d.sql"}

	assert.Empty(t, releaseNotesTimedRunFindings(
		"Each run logged `Successfully applied 2 pending\nmigration(s)`:", applied))
	assert.Equal(t, []string{
		"docs/releases/platform-v1.1.0.md says the timed runs logged `Successfully applied 1 pending migration(s)`; the upgrade applies 2 migrations: time it again",
	}, releaseNotesTimedRunFindings(
		"Upgrading from 1.0.0 applies 2 migrations. The times below are of the first, measured before `101_d.sql` existed."+
			" Each run logged `Successfully applied 1 pending migration(s)`:", applied))
	assert.Equal(t, []string{
		"docs/releases/platform-v1.1.0.md quotes no line the timed runs logged: Successfully applied 2 pending migration(s)",
	}, releaseNotesTimedRunFindings("Upgrading from 1.0.0 applies 2 migrations, timed on a laptop.", applied))
}

// TestPlatform110ReleaseNotesCiteEachSecurityHeading holds the security table
// of the platform 1.1.0 release notes to CHANGELOG.md: one row for each typed
// Security heading of the release, and no row for a heading it does not carry.
func TestPlatform110ReleaseNotesCiteEachSecurityHeading(t *testing.T) {
	requirePublicRepositoryFiles(t)

	section, _ := platform110ChangelogSection(aim03ReadRepoFile(t, "CHANGELOG.md"))
	require.NotEmpty(t, changelogSecurityHeadings(section),
		"CHANGELOG.md has no Security heading for platform 1.1.0; the section scan is broken, not the notes")

	findings := releaseNotesSecurityRowFindings(section, aim03ReadRepoFile(t, platform110ReleaseNotes))
	assert.Empty(t, findings, "give each Security heading of the release one row in the notes' security table:\n%s",
		strings.Join(findings, "\n"))
}

// TestPlatform110ReleaseNotesNameTheMigrationsTheUpgradeApplies holds the
// migration count and range the notes give for the upgrade from 1.0.0, and the
// count the timed runs of that upgrade logged, to the migration files on the
// tree, until CHANGELOG.md has a [1.1.0] section; a migration added after that
// belongs to the next release.
func TestPlatform110ReleaseNotesNameTheMigrationsTheUpgradeApplies(t *testing.T) {
	requirePublicRepositoryFiles(t)

	if _, released := platform110ChangelogSection(aim03ReadRepoFile(t, "CHANGELOG.md")); released {
		t.Skip("CHANGELOG.md has a [1.1.0] section; the notes describe that release")
	}

	entries, err := os.ReadDir(filepath.Join(aim03RepoRoot(t), "apps", "backend", "migrations"))
	require.NoError(t, err)
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	carried, applied := platform100UpgradeMigrations(names)
	require.Len(t, carried, platform100Migrations,
		"the migration files up to %s are not the %d the 1.0.0 tag carries; a file numbered among them is applied by the upgrade too",
		platform100LastMigration, platform100Migrations)

	notes := aim03ReadRepoFile(t, platform110ReleaseNotes)
	findings := append(releaseNotesMigrationRangeFindings(notes, applied), releaseNotesTimedRunFindings(notes, applied)...)
	assert.Empty(t, findings, "name the migrations the upgrade applies, and time it again as the notes' Measured migration time section describes:\n%s",
		strings.Join(findings, "\n"))
}
