package main

import (
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Java SDK's version is written out in more files than its pom: the VERSION
// file that GET /api/v1/sdk/download?sdk=java names the zip after, the strings
// the SDK reports itself with (its User-Agent, the sdkVersion it sends, its SBOM
// component), and the documents that name the artifact `mvn install` places in
// the local Maven repository. A version change that moves the pom and misses one
// of them ships an SDK that reports or documents another version. These tests
// list every file and line that states another version than the pom's.

const (
	javaSDKPomPath     = "sdk/java/pom.xml"
	javaSDKVersionFile = "sdk/java/VERSION"
)

// javaSDKSourceDirs are the repo-relative directories whose .java files are read in full.
var javaSDKSourceDirs = []string{"sdk/java/src/main/java", "sdk/java/examples"}

// javaSDKVersionDocuments are the repo-relative documents that name the Java SDK's artifact or version.
var javaSDKVersionDocuments = []string{
	"README.md",
	"docs/sdk/java.md",
	"docs/use-cases/fleet-governance.md",
	"sdk/java/README.md",
	"examples/a2a-multi-agent-demo/java/pom.xml",
	"apps/web/lib/sdk-tabs.ts",
}

const javaSDKVersion = `(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)`

// javaSDKVersionStatements are the forms in which a file states the Java SDK's
// own version; group 1 is the version. An agent's or an MCP server's version,
// such as `payload.put("version", "1.0.0")`, is not one of them.
var javaSDKVersionStatements = []*regexp.Regexp{
	regexp.MustCompile(`aim-sdk-java@` + javaSDKVersion),
	regexp.MustCompile(`AIM-Java-SDK/` + javaSDKVersion),
	regexp.MustCompile(`org\.opena2a/aim-sdk@` + javaSDKVersion),
	regexp.MustCompile(`org\.opena2a:aim-sdk:` + javaSDKVersion),
	regexp.MustCompile(`"sdkVersion",\s*"` + javaSDKVersion + `"`),
	regexp.MustCompile(`sdkComponent\.version\s*=\s*"` + javaSDKVersion + `"`),
	regexp.MustCompile(`<artifactId>aim-sdk</artifactId>[\s*]*<version>` + javaSDKVersion + `</version>`),
	regexp.MustCompile(`target/aim-sdk-` + javaSDKVersion + `\.jar`),
	regexp.MustCompile(`Version ` + javaSDKVersion + `, built from source`),
	regexp.MustCompile(`Current: ` + javaSDKVersion + `\.`),
}

type javaSDKVersionStatement struct {
	file    string
	line    int
	version string
}

func (s javaSDKVersionStatement) String() string {
	return fmt.Sprintf("%s:%d states %s", s.file, s.line, s.version)
}

// javaSDKVersionStatementsIn returns each statement of the Java SDK's version in text, in file order.
func javaSDKVersionStatementsIn(file, text string) []javaSDKVersionStatement {
	type match struct {
		offset  int
		version string
	}
	var matches []match
	for _, pattern := range javaSDKVersionStatements {
		for _, m := range pattern.FindAllStringSubmatchIndex(text, -1) {
			matches = append(matches, match{offset: m[0], version: text[m[2]:m[3]]})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].offset < matches[j].offset })

	statements := make([]javaSDKVersionStatement, 0, len(matches))
	for _, m := range matches {
		statements = append(statements, javaSDKVersionStatement{
			file:    file,
			line:    strings.Count(text[:m.offset], "\n") + 1,
			version: m.version,
		})
	}
	return statements
}

// javaSDKPomVersion returns the pom's own version: the <version> element directly under <project>.
func javaSDKPomVersion(t *testing.T) string {
	t.Helper()
	var project struct {
		XMLName xml.Name `xml:"project"`
		Version string   `xml:"version"`
	}
	require.NoError(t, xml.Unmarshal([]byte(aim03ReadRepoFile(t, javaSDKPomPath)), &project), "parse %s", javaSDKPomPath)
	version := strings.TrimSpace(project.Version)
	require.NotEmpty(t, version, "%s declares no project version", javaSDKPomPath)
	return version
}

// javaSDKSourceFiles returns the repo-relative .java files under javaSDKSourceDirs, sorted.
func javaSDKSourceFiles(t *testing.T) []string {
	t.Helper()
	// `go test` runs a package's tests in the package directory, apps/backend/cmd/server.
	root := filepath.Join("..", "..", "..", "..")
	_, err := os.Stat(filepath.Join(root, javaSDKPomPath))
	require.NoError(t, err, "the repository root holds %s", javaSDKPomPath)

	var files []string
	for _, dir := range javaSDKSourceDirs {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && strings.HasSuffix(path, ".java") {
				rel, err := filepath.Rel(root, path)
				if err != nil {
					return err
				}
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		require.NoError(t, err, "walk %s", dir)
	}
	sort.Strings(files)
	return files
}

func TestJavaSDKVersionFileStatesThePomVersion(t *testing.T) {
	assert.Equal(t, javaSDKPomVersion(t), strings.TrimSpace(aim03ReadRepoFile(t, javaSDKVersionFile)),
		"%s names the Java SDK download; it has to state the version %s declares", javaSDKVersionFile, javaSDKPomPath)
}

func TestJavaSDKVersionStatementsAgreeWithThePom(t *testing.T) {
	declared := javaSDKPomVersion(t)

	files := append(append([]string{}, javaSDKVersionDocuments...), javaSDKSourceFiles(t)...)
	var fromSources, contradictions []string
	for _, file := range files {
		for _, statement := range javaSDKVersionStatementsIn(file, aim03ReadRepoFile(t, file)) {
			if strings.HasPrefix(file, "sdk/java/src/main/java/") {
				fromSources = append(fromSources, statement.String())
			}
			if statement.version != declared {
				contradictions = append(contradictions, statement.String())
			}
		}
	}

	assert.NotEmpty(t, fromSources, "no file under sdk/java/src/main/java states the SDK version in a form this test "+
		"reads; if the version now comes from one place, point this test at it")
	assert.Empty(t, contradictions, "%s declares %s. These files state another Java SDK version:\n  %s",
		javaSDKPomPath, declared, strings.Join(contradictions, "\n  "))
}

func TestJavaSDKVersionStatementsIn_ReportsEachStatementWithItsLine(t *testing.T) {
	text := "User-Agent: AIM-Java-SDK/1.2.0\n" +
		" *     <artifactId>aim-sdk</artifactId>\n" +
		" *     <version>1.1.0</version>\n" +
		"payload.put(\"version\", \"1.0.0\");\n"

	var got []string
	for _, statement := range javaSDKVersionStatementsIn("doc.md", text) {
		got = append(got, statement.String())
	}
	assert.Equal(t, []string{"doc.md:1 states 1.2.0", "doc.md:2 states 1.1.0"}, got,
		"an agent's or an MCP server's own version is not the SDK's")
}
