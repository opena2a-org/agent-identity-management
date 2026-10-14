package main

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	// apiReferenceEndpoint matches a line of docs/API.md that documents one
	// endpoint: a heading, or a line of a code block, that is a method and a path.
	apiReferenceEndpoint = regexp.MustCompile(`^(?:#{2,6} )?(GET|POST|PUT|PATCH|DELETE) (/[^\s?]+)`)
	// apiReferenceLink matches a markdown link to docs/API.md.
	apiReferenceLink = regexp.MustCompile(`\]\((?:[^)\s]*/)?API\.md(?:#[^)\s]*)?\)`)
	// endpointTotal captures the number, and a trailing plus, in "160 endpoints"
	// and "60+ endpoints".
	endpointTotal = regexp.MustCompile(`(?i)\b(\d+)(\+?)\s+endpoints\b`)

	sqlLineComment = regexp.MustCompile(`(?m)--.*$`)
	// migrationTableStatement matches a statement that adds a table, one that
	// removes tables, and one that renames a table.
	migrationTableStatement = regexp.MustCompile(`(?is)\bCREATE\s+(?:UNLOGGED\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w."]+)` +
		`|\bDROP\s+TABLE\s+(?:IF\s+EXISTS\s+)?([^;]+)` +
		`|\bALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?([\w."]+)\s+RENAME\s+TO\s+([\w."]+)`)
	// tableTotal captures the number in "16 tables", "90+ tables" and "Tables: 16".
	tableTotal = regexp.MustCompile(`(?i)\b(\d+)(\+?)\s+tables\b|\btables:\s*(\d+)(\+?)`)

	// packageCoverageFigure matches a line that gives a backend package and a
	// percentage.
	packageCoverageFigure = regexp.MustCompile("`(?:\\./)?(?:internal|cmd)/[^`]*`.*\\d+(?:\\.\\d+)?%")

	// sdkVersionNamed captures the SDK and the version in "Python SDK v1.14.0",
	// "Java SDK 1.0.0" and "Python SDK Version: 1.14.0".
	sdkVersionNamed = regexp.MustCompile(`\b(Python|Java) SDK(?: Version:)? v?(\d+\.\d+\.\d+)\b`)
	// sdkResultRow captures a row of the test plan's results table: the SDK,
	// the tests run, passed and failed, and the success rate.
	sdkResultRow  = regexp.MustCompile(`^\|\s*(Python|Java) SDK[^|]*\|\s*(\d+)\s*\|\s*(\d+)\s*\|\s*(\d+)\s*\|\s*(\d+(?:\.\d+)?)%\s*\|`)
	docPercentage = regexp.MustCompile(`(\d+(?:\.\d+)?)%`)
	passWord      = regexp.MustCompile(`(?i)\bpass`)
	javaSDKPom    = regexp.MustCompile(`<artifactId>aim-sdk</artifactId>\s*<version>([^<]+)</version>`)
)

// figureHolds reports whether a figure a document gives stands against the
// measured one: "N" must be it, and "N+" at most it.
func figureHolds(number, plus string, measured int) bool {
	n, err := strconv.Atoi(number)
	if err != nil {
		return false
	}
	if plus == "+" {
		return n <= measured
	}
	return n == measured
}

// apiReferenceEndpoints returns how many distinct endpoints the API reference
// documents.
func apiReferenceEndpoints(reference string) int {
	seen := map[string]bool{}
	for _, line := range strings.Split(reference, "\n") {
		if m := apiReferenceEndpoint.FindStringSubmatch(line); m != nil {
			seen[m[1]+" "+m[2]] = true
		}
	}
	return len(seen)
}

// apiReferenceTotalFindings returns one line per place a document links to the
// API reference and says how many endpoints it covers with a number the
// reference does not bear out.
func apiReferenceTotalFindings(name, doc string, documented int) []string {
	var findings []string
	for i, line := range strings.Split(doc, "\n") {
		if !apiReferenceLink.MatchString(line) {
			continue
		}
		for _, m := range endpointTotal.FindAllStringSubmatch(line, -1) {
			if !figureHolds(m[1], m[2], documented) {
				findings = append(findings, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
			}
		}
	}
	return findings
}

// sqlTableName reduces a table name as a statement writes it to the name the
// schema holds: unquoted, lower case, without the public schema prefix.
func sqlTableName(written string) string {
	name := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(written), `"`, ""))
	return strings.TrimPrefix(name, "public.")
}

// migrationTables returns the tables the migrations leave in the schema, read
// from their statements in the order given: a CREATE TABLE adds one, a DROP
// TABLE removes those it lists, and ALTER TABLE ... RENAME TO replaces one.
func migrationTables(migrations []string) map[string]bool {
	tables := map[string]bool{}
	for _, migration := range migrations {
		sql := sqlLineComment.ReplaceAllString(migration, "")
		for _, m := range migrationTableStatement.FindAllStringSubmatch(sql, -1) {
			switch {
			case m[1] != "":
				tables[sqlTableName(m[1])] = true
			case m[2] != "":
				for _, dropped := range strings.Split(m[2], ",") {
					if words := strings.Fields(dropped); len(words) > 0 {
						delete(tables, sqlTableName(words[0]))
					}
				}
			default:
				delete(tables, sqlTableName(m[3]))
				tables[sqlTableName(m[4])] = true
			}
		}
	}
	return tables
}

// tableTotalFindings returns one line per place a document gives a table total
// other than the number of tables the database has.
func tableTotalFindings(name, doc string, tables int) []string {
	var findings []string
	for i, line := range strings.Split(doc, "\n") {
		for _, m := range tableTotal.FindAllStringSubmatch(line, -1) {
			number, plus := m[1], m[2]
			if number == "" {
				number, plus = m[3], m[4]
			}
			if !figureHolds(number, plus, tables) {
				findings = append(findings, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
			}
		}
	}
	return findings
}

// packageCoverageFindings returns one line per place a document gives a
// coverage percentage for a backend package.
func packageCoverageFindings(name, doc string) []string {
	var findings []string
	for i, line := range strings.Split(doc, "\n") {
		if packageCoverageFigure.MatchString(line) {
			findings = append(findings, fmt.Sprintf("%s:%d: %s", name, i+1, strings.TrimSpace(line)))
		}
	}
	return findings
}

// versionAfter reports whether dotted version a is later than b.
func versionAfter(a, b string) bool {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, _ := strconv.Atoi(as[i])
		y, _ := strconv.Atoi(bs[i])
		if x != y {
			return x > y
		}
	}
	return len(as) > len(bs)
}

// sdkTestPlanFindings returns one line per place the SDK test plan names an SDK
// version later than the one that SDK is at, has a results row whose counts and
// success rate disagree, or gives an SDK a pass rate other than its row's.
// current maps "Python" and "Java" to the version each SDK is at.
func sdkTestPlanFindings(name, plan string, current map[string]string) []string {
	lines := strings.Split(plan, "\n")
	var findings []string
	finding := func(i int, why string) {
		findings = append(findings, fmt.Sprintf("%s:%d: %s: %s", name, i+1, why, strings.TrimSpace(lines[i])))
	}

	rates := map[string]float64{}
	for i, line := range lines {
		for _, m := range sdkVersionNamed.FindAllStringSubmatch(line, -1) {
			if versionAfter(m[2], current[m[1]]) {
				finding(i, fmt.Sprintf("the %s SDK is at %s", m[1], current[m[1]]))
			}
		}
		m := sdkResultRow.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		tests, _ := strconv.Atoi(m[2])
		passed, _ := strconv.Atoi(m[3])
		failed, _ := strconv.Atoi(m[4])
		stated, _ := strconv.ParseFloat(m[5], 64)
		if tests == 0 || passed+failed != tests {
			finding(i, "passed and failed do not add up to the tests run")
			continue
		}
		rates[m[1]] = math.Round(float64(passed)*1000/float64(tests)) / 10
		if stated != rates[m[1]] {
			finding(i, fmt.Sprintf("%d of %d is %v%%", passed, tests, rates[m[1]]))
		}
	}

	for i, line := range lines {
		if sdkResultRow.MatchString(line) || !passWord.MatchString(line) {
			continue
		}
		var sdks []string
		both := strings.Contains(line, "Both SDKs")
		for _, sdk := range []string{"Python", "Java"} {
			if both || strings.Contains(line, sdk+" SDK") {
				sdks = append(sdks, sdk)
			}
		}
		for _, m := range docPercentage.FindAllStringSubmatch(line, -1) {
			stated, _ := strconv.ParseFloat(m[1], 64)
			for _, sdk := range sdks {
				rate, ok := rates[sdk]
				if !ok {
					finding(i, fmt.Sprintf("the results table has no %s SDK row", sdk))
				} else if stated != rate {
					finding(i, fmt.Sprintf("the results table gives the %s SDK %v%%", sdk, rate))
				}
			}
		}
	}
	return findings
}

func TestDocsFigureFindings(t *testing.T) {
	t.Run("API reference totals", func(t *testing.T) {
		reference := strings.Join([]string{
			"#### POST /auth/login",
			"#### GET /api/v1/agents",
			"```http",
			"POST /api/v1/agents/{id}/verify/challenge",
			"GET /api/v1/agents?status=verified",
			"```",
			"Send a POST /api/v1/agents request.",
		}, "\n")
		assert.Equal(t, 3, apiReferenceEndpoints(reference))

		doc := strings.Join([]string{
			"3. **[API Reference](../API.md)** - All 160 endpoints",
			"- **[Full API Reference](../API.md#agents-endpoints)** - 3 endpoints documented",
			"- [API](./API.md) - 2+ endpoints",
			"- [API](./API.md) - 4+ endpoints",
			"The server has 160 endpoints.",
			"- [Summary](./API_ENDPOINT_SUMMARY.md) - 62 endpoints",
		}, "\n")
		assert.Equal(t, []string{
			"docs/tutorials/README.md:1: 3. **[API Reference](../API.md)** - All 160 endpoints",
			"docs/tutorials/README.md:4: - [API](./API.md) - 4+ endpoints",
		}, apiReferenceTotalFindings("docs/tutorials/README.md", doc, 3))
	})

	t.Run("table totals", func(t *testing.T) {
		tables := migrationTables([]string{
			"-- CREATE TABLE never_created (id INT);\nCREATE TABLE users (id INT);\nCREATE TABLE IF NOT EXISTS public.\"Agents\" (id INT);",
			"CREATE TABLE IF NOT EXISTS users (id INT);\ncreate table scratch (id INT);\nCREATE TEMP TABLE staging (id INT);",
			"DROP TABLE IF EXISTS scratch, missing CASCADE;\nALTER TABLE agents RENAME TO workloads;\nALTER TABLE users RENAME COLUMN id TO user_id;",
		})
		assert.Equal(t, map[string]bool{"users": true, "workloads": true}, tables)

		doc := strings.Join([]string{
			"- [x] Database (PostgreSQL, 16 tables)",
			"- Tables: 16",
			"- [x] Database (PostgreSQL, 2 tables)",
			"- Tables: 2",
			"More than 1+ tables, never 3+ tables.",
		}, "\n")
		assert.Equal(t, []string{
			"guide.md:1: - [x] Database (PostgreSQL, 16 tables)",
			"guide.md:2: - Tables: 16",
			"guide.md:5: More than 1+ tables, never 3+ tables.",
		}, tableTotalFindings("guide.md", doc, len(tables)))
	})

	t.Run("package coverage figures", func(t *testing.T) {
		doc := strings.Join([]string{
			"| `internal/domain` | 97.5% |",
			"`./internal/application/...` is at 33%",
			"- **Core packages** (`domain`, `infrastructure/*`): 80%+",
			"go test -cover ./internal/domain/",
		}, "\n")
		assert.Equal(t, []string{
			"README.md:1: | `internal/domain` | 97.5% |",
			"README.md:2: `./internal/application/...` is at 33%",
		}, packageCoverageFindings("README.md", doc))
	})

	t.Run("SDK test plan", func(t *testing.T) {
		assert.True(t, versionAfter("1.1.0", "1.0.0"))
		assert.True(t, versionAfter("2.0.0", "1.14.9"))
		assert.False(t, versionAfter("1.14.0", "2.0.3"))
		assert.False(t, versionAfter("1.0.0", "1.0.0"))

		plan := strings.Join([]string{
			"| Python SDK v1.14.0 | 19 | 19 | 0 | 100% |",
			"| Java SDK v1.1.0 | 19 | 18 | 1 | 100% |",
			"Both SDKs achieve 100% pass rate.",
			"2. **Use Python SDK** for Python workloads (94.7% pass rate)",
			"### Python SDK (100% - 19/19 PASS)",
			"The Python SDK needs Python 3.10+.",
			"*Java SDK Version: 1.0.0*",
		}, "\n")
		assert.Equal(t, []string{
			"plan.md:2: the Java SDK is at 1.0.0: | Java SDK v1.1.0 | 19 | 18 | 1 | 100% |",
			"plan.md:2: 18 of 19 is 94.7%: | Java SDK v1.1.0 | 19 | 18 | 1 | 100% |",
			"plan.md:3: the results table gives the Java SDK 94.7%: Both SDKs achieve 100% pass rate.",
			"plan.md:4: the results table gives the Python SDK 100%: 2. **Use Python SDK** for Python workloads (94.7% pass rate)",
		}, sdkTestPlanFindings("plan.md", plan, map[string]string{"Python": "2.0.3", "Java": "1.0.0"}))
	})
}

// TestDocumentedAPIReferenceTotalsAreItsOwn holds every markdown line that
// links to docs/API.md and says how many endpoints it covers to the number of
// endpoints that file documents. Two tutorials said it documents all 160; it
// documents a fraction of the routes the server mounts.
func TestDocumentedAPIReferenceTotalsAreItsOwn(t *testing.T) {
	requirePublicRepositoryFiles(t)

	documented := apiReferenceEndpoints(aim03ReadRepoFile(t, "docs/API.md"))
	require.NotZero(t, documented, "docs/API.md documents no endpoint; the endpoint scan is broken, not the documents")

	root := aim03RepoRoot(t)
	links := 0
	var findings []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p != root && docsWalkSkip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") || d.Name() == "CHANGELOG.md" {
			return nil
		}
		src, err := os.ReadFile(p) //nolint:gosec // G304: walking the repository's own documents
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		links += len(apiReferenceLink.FindAllString(string(src), -1))
		findings = append(findings, apiReferenceTotalFindings(filepath.ToSlash(rel), string(src), documented)...)
		return nil
	})
	require.NoError(t, err)
	require.NotZero(t, links, "no document links to docs/API.md; the document scan is broken, not the documents")

	assert.Empty(t, findings, "docs/API.md documents %d endpoints; a line that links to it and counts them must say so:\n%s",
		documented, strings.Join(findings, "\n"))
}

// TestDeploymentReferenceTableTotalIsTheDatabases holds any table total the
// deployment reference gives to the tables a migrated database has: the ones
// the migrations create, and the migration runner's own schema_migrations.
func TestDeploymentReferenceTableTotalIsTheDatabases(t *testing.T) {
	requirePublicRepositoryFiles(t)

	root := aim03RepoRoot(t)
	dir := filepath.Join(root, "apps", "backend", "migrations")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var migrations []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // G304: the repository's own migrations
		require.NoError(t, err)
		migrations = append(migrations, string(src))
	}
	tables := migrationTables(migrations)
	require.NotEmpty(t, tables, "the migrations create no table; the statement scan is broken, not the document")
	require.False(t, tables["schema_migrations"], "a migration creates schema_migrations; count it once")
	require.Contains(t, aim03ReadRepoFile(t, "apps/backend/cmd/server/main.go"), "CREATE TABLE IF NOT EXISTS schema_migrations",
		"the migration runner no longer creates schema_migrations")
	total := len(tables) + 1

	const guide = "docs/guides/QUICK_DEPLOYMENT_REFERENCE.md"
	findings := tableTotalFindings(guide, aim03ReadRepoFile(t, guide), total)
	assert.Empty(t, findings, "%d migrations create %d tables and the migration runner one more, %d in all:\n%s",
		len(migrations), len(tables), total, strings.Join(findings, "\n"))
}

// TestBackendReadmeGivesNoPackageCoverageFigure keeps per-package coverage
// percentages out of the backend README. A coverage figure is the output of a
// package's whole suite run under -cover, so no test can hold the README to
// one, and the table it carried had drifted from what the command prints.
// The README says how to print the current figures instead.
func TestBackendReadmeGivesNoPackageCoverageFigure(t *testing.T) {
	requirePublicRepositoryFiles(t)

	const readme = "apps/backend/README.md"
	doc := aim03ReadRepoFile(t, readme)
	require.Contains(t, doc, "## Test Coverage", "the backend README has no coverage section")
	assert.Regexp(t, regexp.MustCompile(`go test \S+ -cover\b`), doc, "the README shows the command that prints coverage")

	findings := packageCoverageFindings(readme, doc)
	assert.Empty(t, findings, "the README gives a coverage percentage for a package; give the command that prints it:\n%s",
		strings.Join(findings, "\n"))
}

// TestSDKTestPlanVersionsAndPassRatesHold holds the SDK test plan to the SDKs
// and to itself: it names no SDK version later than the one the SDK is at, and
// every pass rate it gives an SDK is the one its results table gives. The plan
// named Java SDK 1.1.0 while sdk/java/pom.xml said 1.0.0, and gave the Python
// SDK both 100% and 94.7%.
func TestSDKTestPlanVersionsAndPassRatesHold(t *testing.T) {
	requirePublicRepositoryFiles(t)

	python := strings.TrimSpace(aim03ReadRepoFile(t, "sdk/python/VERSION"))
	require.Regexp(t, `^\d+\.\d+\.\d+$`, python, "sdk/python/VERSION holds no version")
	pom := javaSDKPom.FindStringSubmatch(aim03ReadRepoFile(t, "sdk/java/pom.xml"))
	require.NotNil(t, pom, "sdk/java/pom.xml gives the aim-sdk artifact no version")

	const plan = "docs/SDK_TEST_PLAN.md"
	doc := aim03ReadRepoFile(t, plan)
	require.NotEmpty(t, sdkVersionNamed.FindAllString(doc, -1), "the test plan names no SDK version; the scan is broken, not the plan")

	findings := sdkTestPlanFindings(plan, doc, map[string]string{"Python": python, "Java": strings.TrimSpace(pom[1])})
	assert.Empty(t, findings, "the SDK test plan must agree with the SDKs (Python %s, Java %s) and with its own results table:\n%s",
		python, pom[1], strings.Join(findings, "\n"))
}
