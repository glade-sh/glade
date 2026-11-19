package repoguard

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// The repository guards consume shared scans: one decode of every tracked
// asset feeds both public-data rulesets, and one parse of every in-scope Go
// file feeds both structural guards. These regressions plant violations and
// duplicated work through those scans, so a consolidation cannot silently
// drop a guard's detection or keep decoding and parsing twice.

func publicScanPlantedCatalog(t *testing.T) string {
	t.Helper()
	var catalog bytes.Buffer
	writer := gzip.NewWriter(&catalog)
	if _, err := writer.Write([]byte(`{"email":"person` + `@gmail.com","review_note":"tracked in ` + "RO" + `AD-12"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return catalog.String()
}

func TestPublicDataScanReportsPlantedViolations(t *testing.T) {
	sum := func(name string) string {
		digest := sha256.Sum256([]byte(campaignNameSalt + name))
		return hex.EncodeToString(digest[:])
	}
	names := campaignNameMatcher([]campaignName{
		{campaignNamePrefix, len("glade-planted-"), sum("glade-planted-")},
	})
	catalog := publicScanPlantedCatalog(t)
	files := []repoScanFile{
		{rel: "docs/leak.md", text: "contact person" + "@gmail.com about " + "ROAD-" + "12\n"},
		{rel: "docs/glade-planted-x/notes.md", text: "notes\n"},
		{rel: "docs/catalog.json.gz", text: catalog},
		{rel: "docs/broken.json.gz", text: catalog[:len(catalog)-8]},
		{rel: "docs/plans/ORG-RUN.txt", text: "capture " + "ROAD-" + "12 person" + "@gmail.com\n"},
	}
	scans := scanTrackedFilesForPublicData(files, names, scanPublicAsset)
	if len(scans) != len(files) {
		t.Fatalf("scan entries = %d, want %d", len(scans), len(files))
	}
	if got := scans[0].release; len(got) != 1 || got[0].line != 1 || got[0].kind != "personal-provider mailbox" {
		t.Fatalf("release findings on planted mailbox = %v, want one personal-provider mailbox on line 1", got)
	}
	if got := scans[0].campaign; len(got) != 1 || got[0].line != 1 || got[0].kind != "campaign roadmap reference" {
		t.Fatalf("campaign findings on planted ticket = %v, want one campaign roadmap reference on line 1", got)
	}
	if got := scans[1].campaignPath; len(got) != 1 || got[0] != "private org alias or host name" {
		t.Fatalf("campaign path findings on planted alias = %v, want one private org alias or host name", got)
	}
	if got := scans[2].release; len(got) != 1 || got[0].member != " (member 1)" || got[0].kind != "personal-provider mailbox" {
		t.Fatalf("release findings on planted gzip member = %v, want one mailbox in member 1", got)
	}
	if got := scans[2].campaign; len(got) != 1 || got[0].member != " (member 1)" || got[0].kind != "campaign roadmap reference" {
		t.Fatalf("campaign findings on planted gzip member = %v, want one roadmap reference in member 1", got)
	}
	if scans[2].err != nil {
		t.Fatalf("planted gzip member decode failed: %v", scans[2].err)
	}
	if scans[3].err == nil {
		t.Fatal("planted corrupt asset did not fail the shared decode")
	}
	if len(scans[3].release) != 0 || len(scans[3].campaign) != 0 {
		t.Fatalf("corrupt asset produced findings %v %v; want only the decode error", scans[3].release, scans[3].campaign)
	}
	if !scans[4].privateCapture {
		t.Fatal("planted ORG-RUN.txt was not recognized as private capture material")
	}
	if got := scans[4].release; len(got) != 1 || got[0].kind != "personal-provider mailbox" {
		t.Fatalf("release findings inside private capture file = %v, want the planted mailbox", got)
	}
	if len(scans[4].campaignPath) != 0 || len(scans[4].campaign) != 0 {
		t.Fatalf("campaign findings on private capture file = %v %v; want none (the campaign guard skips these files)", scans[4].campaignPath, scans[4].campaign)
	}
}

func TestPublicDataScanDecodesEachAssetOnce(t *testing.T) {
	files := []repoScanFile{
		{rel: "docs/one.json.gz", text: publicScanPlantedCatalog(t)},
		{rel: "docs/two.md", text: "person" + "@gmail.com and " + "ROAD-" + "12\n"},
	}
	decodes := 0
	scans := scanTrackedFilesForPublicData(files, campaignNameMatcher(nil), func(rel, text string, visit func(string, string)) error {
		decodes++
		return scanPublicAsset(rel, text, visit)
	})
	if decodes != len(files) {
		t.Fatalf("asset decodes = %d, want %d: one shared decode must feed both public-data rulesets", decodes, len(files))
	}
	for _, scan := range scans {
		if len(scan.release) != 1 || len(scan.campaign) != 1 {
			t.Fatalf("one decode must feed both rulesets: %+v", scan)
		}
	}
}

const recordsWriteOutsideFunnelSource = `package storage

type State struct {
	Records map[string]string
}

func put(object State, id string) {
	object.Records[id] = "x"
}
`

const cookieSinkUseSource = `package fixture

import "net/http"

func probe() {
	http.SetCookie(nil, nil)
}
`

const publicSessionCookieSource = `package playground

import "net/http"

func session() {
	http.SetCookie(nil, &http.Cookie{Name: "glade_session", Value: "owned", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}
`

func writeStructuralScanFile(t *testing.T, root, rel, source string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStructuralRepoScanReportsPlantedViolations(t *testing.T) {
	root := t.TempDir()
	writeStructuralScanFile(t, root, "internal/server/unreviewed.go", cookieSinkUseSource)
	writeStructuralScanFile(t, root, "internal/playground/public_sessions.go", publicSessionCookieSource)
	writeStructuralScanFile(t, root, "internal/storage/leaky.go", recordsWriteOutsideFunnelSource)
	writeStructuralScanFile(t, root, "internal/storage/testdata/leaky.go", recordsWriteOutsideFunnelSource)
	writeStructuralScanFile(t, root, "internal/storage/record_writes.go", recordsWriteOutsideFunnelSource)
	writeStructuralScanFile(t, root, "internal/third_party/vendored.go", recordsWriteOutsideFunnelSource)
	writeStructuralScanFile(t, root, "internal/broken/main.go", "package broken\n\nfunc ( {\n")
	writeStructuralScanFile(t, root, "site/broken.go", "package site\n\nfunc ( {\n")
	writeStructuralScanFile(t, root, "site/untracked_cookie.go", cookieSinkUseSource)

	scans, err := scanWorkingTreeGoFiles(root, parseRepoGoFileForStructuralGuards)
	if err != nil {
		t.Fatal(err)
	}
	byRel := map[string]structuralRepoFileScan{}
	for _, scan := range scans {
		byRel[scan.rel] = scan
	}
	if got := byRel["internal/server/unreviewed.go"].cookieUses; len(got) != 1 || got[0] != "internal/server/unreviewed.go: http.SetCookie" {
		t.Fatalf("cookie uses on planted sink = %v, want the unreviewed SetCookie", got)
	}
	if got := byRel["site/untracked_cookie.go"].cookieUses; len(got) != 1 || got[0] != "site/untracked_cookie.go: http.SetCookie" {
		t.Fatalf("cookie uses on planted non-product sink = %v, want the collected SetCookie", got)
	}
	if sink := byRel["internal/playground/public_sessions.go"].cookieSink; sink == nil ||
		sink.literals != 1 || sink.setCookieCalls != 1 || !sink.safe {
		t.Fatalf("public session cookie scan = %+v, want one reviewed literal, one SetCookie sink and pinned flags", byRel["internal/playground/public_sessions.go"].cookieSink)
	}
	if got := byRel["internal/storage/leaky.go"].recordWrites; len(got) != 1 || got[0] != "8: direct write: index assignment" {
		t.Fatalf("record funnel findings on planted write = %v, want one direct write at line 8", got)
	}
	if scan := byRel["internal/storage/testdata/leaky.go"]; scan.recordScanned || len(scan.recordWrites) != 0 {
		t.Fatalf("record funnel scanned testdata = %+v; testdata stays outside its scope", scan)
	}
	if scan := byRel["internal/storage/record_writes.go"]; scan.recordScanned || len(scan.recordWrites) != 0 {
		t.Fatalf("record funnel scanned its own implementation = %+v; the funnel file stays exempt", scan)
	}
	if got := byRel["internal/third_party/vendored.go"].recordWrites; len(got) != 1 {
		t.Fatalf("record funnel findings inside a cookie-skipped directory = %v, want the planted write kept in funnel scope", got)
	}
	if scan := byRel["internal/third_party/vendored.go"]; len(scan.cookieUses) != 0 {
		t.Fatalf("cookie review scanned third_party = %v; its walk skips that directory", scan.cookieUses)
	}
	if scan := byRel["internal/broken/main.go"]; scan.parseErr == nil || !scan.recordScanned {
		t.Fatalf("broken file in funnel scope = %+v; its parse error must fail the funnel guard", scan)
	}
	if scan := byRel["site/broken.go"]; scan.parseErr == nil || scan.recordScanned {
		t.Fatalf("broken file outside funnel scope = %+v; the funnel must ignore out-of-scope parse errors", scan)
	}
}

func TestStructuralRepoScanParsesEachGoFileOnce(t *testing.T) {
	root := t.TempDir()
	writeStructuralScanFile(t, root, "internal/storage/leaky.go", recordsWriteOutsideFunnelSource)
	writeStructuralScanFile(t, root, "site/cookie.go", cookieSinkUseSource)
	parses := 0
	scans, err := scanWorkingTreeGoFiles(root, func(positions *token.FileSet, path string) (*ast.File, error) {
		parses++
		return parser.ParseFile(positions, path, nil, parser.SkipObjectResolution)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(scans) != 2 {
		t.Fatalf("scanned files = %d, want 2", len(scans))
	}
	if parses != len(scans) {
		t.Fatalf("Go file parses = %d, want %d: one shared parse must feed every structural guard", parses, len(scans))
	}
	if len(scans[0].recordWrites) != 1 || len(scans[1].cookieUses) != 1 {
		t.Fatalf("one parse must feed both guards: %+v %+v", scans[0], scans[1])
	}
}
