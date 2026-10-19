package handlers

import (
	"archive/zip"
	"bytes"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The guides below have the reader download the SDK from the dashboard, to install it or to
// get fresh credentials. Each names the dashboard page, the download button and the archive,
// and each of those is pinned here to the dashboard's navigation and SDK page and to the file
// name GET /api/v1/sdk/download sends, so a rename on either side fails next to the guides that
// would then be wrong. docs/quick-start.md is held to the same three by the Python SDK's
// tests/test_readme_self_hosted_sign_in.py. A guide that unzips an archive extracts the newest
// one, and a guide that copies credentials from an archive replaces ~/.aim/sdk_credentials.json
// with the archive's copy.
//
// The guides sent the reader to Settings → SDK Download or Dashboard → SDK Download, which the
// navigation has no entry for, to a "Download SDK" button, and to unzip aim-sdk-python.zip,
// which is not the name the server sends (#799, #805).
var sdkInstallGuides = []string{
	"../../docs/sdk/python.md",
	"../../docs/sdk/java.md",
	"../../docs/DEMO_SCRIPT_ECHOLEAK.md",
	"../../docs/tutorials/README.md",
	"../../docs/tutorials/sdk-quickstart.md",
	"../../docs/tutorials/dashboard-walkthrough.md",
	"../../docs/security/token-rotation.md",
	"../../docs/troubleshooting/README.md",
	"../../docs/troubleshooting/authentication.md",
	"../../sdk/java/README.md",
}

// sdkPageTrail is how a guide reaches the SDK page: the navigation's Developers entry, then
// that hub's SDK & docs tab.
const sdkPageTrail = "Developers → SDK & docs"

var (
	// The SDK page labels each download button this way while it is not downloading.
	sdkPageDownloadButton = regexp.MustCompile(`selectedSDK === '([a-z]+)' \? 'Downloading\.\.\.' : '([^']+)'`)
	// A button a guide names: a quoted label from Download to SDK.
	guideDownloadButton = regexp.MustCompile(`"(Download[^"\n]*SDK)"`)
	// An archive a guide names.
	guideArchiveName = regexp.MustCompile(`aim-sdk-[A-Za-z0-9._<>*-]*\.zip`)

	// A line of a guide that unzips an SDK archive, and the one form it may take. A browser saves
	// a second download of an archive as "<name> (1).zip", so a glob such as
	// ~/Downloads/aim-sdk-python-v*.zip can pass unzip two archives; unzip then reads the second
	// as a member to extract from the first, extracts nothing and exits 11 (#805).
	guideUnzipLine     = regexp.MustCompile(`(?m)^.*\bunzip\b.*aim-sdk-.*$`)
	newestArchiveUnzip = regexp.MustCompile(`^unzip "\$\(ls -t (?:~/Downloads/)?aim-sdk-[a-z]+-v\*\.zip \| head -n 1\)"$`)

	// A line of a guide that copies from an archive's .aim folder, and the one form it may take.
	// cp -r of the folder into an existing ~/.aim puts it at ~/.aim/.aim and leaves the old
	// ~/.aim/sdk_credentials.json, the first file both SDKs read, in place (#805).
	guideAimCopyLine = regexp.MustCompile(`(?m)^.*\bcp\b.*aim-sdk-[a-z]+/\.aim.*$`)
	credentialsCopy  = regexp.MustCompile(`^mkdir -p ~/\.aim && cp aim-sdk-[a-z]+/\.aim/sdk_credentials\.json ~/\.aim/sdk_credentials\.json$`)
)

// hubTabList returns the tab list of one hub in the dashboard's hub-tabs.ts.
func hubTabList(t *testing.T, hub string) string {
	t.Helper()
	_, rest, found := strings.Cut(readBackendFile(t, "../web/lib/hub-tabs.ts"), "\n  "+hub+": [\n")
	require.True(t, found, "hub-tabs.ts has no %s hub", hub)
	list, _, found := strings.Cut(rest, "\n  ],")
	require.True(t, found, "the %s hub's tab list does not end", hub)
	return list
}

func TestSDKInstallGuidesNameTheDashboardPageButtonAndArchive(t *testing.T) {
	requirePublicRepositoryFiles(t)

	// The page: the navigation's Developers entry opens a hub whose SDK & docs tab is
	// /dashboard/sdk.
	textHas(t, readBackendFile(t, "../web/lib/navigation.ts"),
		`{ key: "developers", name: "Developers", href: "/dashboard/developers"`, "the navigation's Developers entry")
	textHas(t, hubTabList(t, "developers"), `{ name: "SDK & docs", href: "/dashboard/sdk"`, "the Developers hub's SDK & docs tab")

	// The buttons on that page, by the SDK each one downloads.
	buttons := map[string]string{}
	labels := map[string]bool{}
	for _, m := range sdkPageDownloadButton.FindAllStringSubmatch(readBackendFile(t, "../web/app/dashboard/sdk/page.tsx"), -1) {
		buttons[m[1]] = m[2]
		labels[m[2]] = true
	}

	// The archives: the name the server sends for each SDK, which a guide writes with the
	// version as * in a command and as <version> in prose, and the folder it unpacks into.
	archives := map[string]bool{}
	for _, sdk := range []string{"python", "java"} {
		require.NotEmpty(t, buttons[sdk], "the SDK page has no %s download button", sdk)

		status, header, body := fetchSDKDownload(t, sdk)
		require.Equal(t, fiber.StatusOK, status, string(body))
		version := strings.TrimSpace(readBackendFile(t, "../../sdk/"+sdk+"/VERSION"))
		prefix := "aim-sdk-" + sdk + "-v"
		require.Equal(t, "attachment; filename="+prefix+version+".zip", header.Get("Content-Disposition"),
			"the %s archive's name", sdk)
		archives[prefix+"*.zip"] = true
		archives[prefix+"<version>.zip"] = true

		zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
		require.NoError(t, err, "the body is a zip")
		folder := "aim-sdk-" + sdk + "/"
		for _, e := range zr.File {
			require.True(t, strings.HasPrefix(e.Name, folder), "%s is outside the archive's %s folder", e.Name, folder)
		}
	}
	sent := slices.Sorted(maps.Keys(archives))

	// The unzip and copy lines read across the guides, so neither check passes having read none.
	unzips, copies := 0, 0
	for _, rel := range sdkInstallGuides {
		t.Run(strings.TrimPrefix(rel, "../../"), func(t *testing.T) {
			guide := readBackendFile(t, rel)
			textLacks(t, guide, "→ SDK Download", "a page the navigation has no entry for")
			textHas(t, guide, sdkPageTrail, "the navigation's entry and tab for the SDK page")
			textHas(t, guide, "/dashboard/sdk", "the SDK page's path")

			named := guideDownloadButton.FindAllStringSubmatch(guide, -1)
			assert.NotEmpty(t, named, "the guide names no download button")
			for _, m := range named {
				assert.True(t, labels[m[1]], "the guide names a %q button; the SDK page's are %v", m[1], buttons)
			}

			files := guideArchiveName.FindAllString(guide, -1)
			assert.NotEmpty(t, files, "the guide names no archive")
			for _, f := range files {
				assert.True(t, archives[f], "the guide names %s; the server's archives are written %v", f, sent)
			}

			for _, line := range guideUnzipLine.FindAllString(guide, -1) {
				unzips++
				assert.Regexp(t, newestArchiveUnzip, strings.TrimSpace(line),
					"the guide unzips every archive a glob matches, not the newest one")
			}
			for _, line := range guideAimCopyLine.FindAllString(guide, -1) {
				copies++
				assert.Regexp(t, credentialsCopy, strings.TrimSpace(line),
					"the guide copies the archive's .aim folder, not its sdk_credentials.json over ~/.aim/sdk_credentials.json")
			}
		})
	}
	assert.NotZero(t, unzips, "no guide unzips an SDK archive")
	assert.NotZero(t, copies, "no guide copies credentials from an SDK archive")
}
