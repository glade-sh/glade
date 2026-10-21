package repoguard

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/project"
)

type repoScanFile struct {
	rel  string
	text string
}

type repoScanSnapshot struct {
	paths []string
	files []repoScanFile
	byRel map[string]repoScanFile
}

type repoScanCache struct {
	once  sync.Once
	build func() (repoScanSnapshot, error)
	value repoScanSnapshot
	err   error
}

type repoScanBuilder struct {
	inventory func(string) ([]string, error)
	stat      func(string) (os.FileInfo, error)
	readFile  func(string) ([]byte, error)
}

func (cache *repoScanCache) snapshot() (repoScanSnapshot, error) {
	cache.once.Do(func() {
		cache.value, cache.err = cache.build()
	})
	return cache.value, cache.err
}

var canonicalRepoRoot = repositoryRootPath()

var canonicalRepoScan = repoScanCache{build: func() (repoScanSnapshot, error) {
	return buildRepoScan(canonicalRepoRoot)
}}

func TestNoCorpusSpecificReferencesInRuntimeCode(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range repoGoFiles(t, root) {
		if allowsCorpusTerms(rel) {
			continue
		}
		text := strings.ToLower(readRepoFile(t, root, rel))
		for _, term := range forbiddenCorpusTerms() {
			if strings.Contains(text, term) {
				t.Errorf("%s contains forbidden corpus term %q", rel, term)
			}
		}
	}
}

func TestNoPrivateExamplePackageReferences(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range repoExistingTrackedPaths(t, root) {
		checkPrivateExamplePackageText(t, rel, rel)
	}
	for _, file := range repoRegularFiles(t, root) {
		if strings.IndexByte(file.text, 0) >= 0 {
			continue
		}
		checkPrivateExamplePackageText(t, file.rel, file.text)
	}
}

func TestNoPersonalAbsolutePathsInTrackedText(t *testing.T) {
	root := repoRoot(t)
	forbidden := "/Users/" + "matt"
	for _, file := range repoRegularFiles(t, root) {
		if strings.IndexByte(file.text, 0) >= 0 {
			continue
		}
		if strings.Contains(file.text, forbidden) {
			t.Errorf("%s contains a personal absolute path", file.rel)
		}
	}
}

func TestNoPrivateOrgAliasesInPublicDocs(t *testing.T) {
	privateAlias := regexp.MustCompile(`(?i)\b(?:` + strings.Join([]string{"n" + "u", "n" + "c", "n" + "ams", hyphen("sf", "cred")}, "|") + `)\b`)
	for _, file := range repoRegularFiles(t, repoRoot(t)) {
		if !isHumanFacingExampleSurface(file.rel) || strings.IndexByte(file.text, 0) >= 0 {
			continue
		}
		if match := privateAlias.FindString(file.text); match != "" {
			t.Errorf("%s contains private org alias %q", file.rel, match)
		}
	}
}

func TestPublicExampleNamesAvoidGenericPlaceholders(t *testing.T) {
	root := repoRoot(t)
	for _, file := range repoRegularFiles(t, root) {
		if !isHumanFacingExampleSurface(file.rel) {
			continue
		}
		if strings.IndexByte(file.text, 0) >= 0 {
			continue
		}
		text := file.text
		for _, term := range genericExampleNameTerms() {
			if strings.Contains(text, term) {
				t.Errorf("%s contains generic example name %q; use macrodata-apex, RefinementService, FileRow, or refinement-local instead", file.rel, term)
			}
		}
	}
}

func TestNoAgentPlanningDocsInReleaseSurface(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range repoExistingTrackedPaths(t, root) {
		if strings.HasPrefix(rel, "docs/superpowers/") {
			t.Errorf("%s is an agent planning artifact; keep release docs product-facing", rel)
		}
	}
	for _, file := range repoRegularFiles(t, root) {
		if strings.HasPrefix(file.rel, "docs/superpowers/") || (!strings.HasPrefix(file.rel, "docs/") && !strings.HasPrefix(file.rel, "site/docs-src/")) {
			continue
		}
		if strings.IndexByte(file.text, 0) >= 0 {
			continue
		}
		if strings.Contains(file.text, "For agentic workers") {
			t.Errorf("%s contains agent planning instructions; keep release docs product-facing", file.rel)
		}
	}
}

func TestCaseInsensitiveEqualityAvoidsToLower(t *testing.T) {
	root := repoRoot(t)
	pattern := regexp.MustCompile(`strings\.ToLower\([^\n]+\)\s*(?:==|!=)|(?:==|!=)\s*strings\.ToLower\(`)
	for _, rel := range repoGoFiles(t, root) {
		for _, match := range pattern.FindAllString(readRepoFile(t, root, rel), -1) {
			// Preserve this existing lowercase-normalized form-key comparison until
			// an org-backed change establishes broader Unicode simple-fold behavior.
			if rel == "internal/visualforce/form_binding.go" && match == "strings."+"ToLower(formFieldBindingName(key)) !=" {
				continue
			}
			t.Errorf("%s uses ToLower for equality; use strings.EqualFold: %s", rel, match)
		}
	}
}

func TestNoStringByteLengthAllocation(t *testing.T) {
	root := repoRoot(t)
	needle := "len(" + "[]byte("
	for _, rel := range repoGoFiles(t, root) {
		if strings.Contains(readRepoFile(t, root, rel), needle) {
			t.Errorf("%s converts a string to bytes just to count it; use len(string) or reuse a byte slice", rel)
		}
	}
}

// storage's object-name cache treats an Objects map with an unchanged count as
// holding unchanged names, so product code removes objects through
// storage.DeleteObject, which drops that cache.
func TestOrgObjectDeletesDropObjectNameCache(t *testing.T) {
	root := repoRoot(t)
	pattern := regexp.MustCompile(`\bdelete\([^,()]*\bObjects,`)
	allowed := map[string]string{
		"internal/storage/object_name_index.go": "delete(org.Objects,",
		// A private copy of the map, cloned into a fresh cache before any lookup.
		"internal/apextest/runtime_patch.go": "delete(ambientOrg.Objects,",
	}
	for _, rel := range repoGoFiles(t, root) {
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		for _, match := range pattern.FindAllString(readRepoFile(t, root, rel), -1) {
			if allowed[rel] == match {
				continue
			}
			t.Errorf("%s deletes from an org's Objects map directly; use storage.DeleteObject: %s", rel, match)
		}
	}
}

func TestReleaseBuildProducesParserCapableArtifacts(t *testing.T) {
	root := repoRoot(t)
	text := readRepoFile(t, root, "scripts/release-build.sh")
	if strings.Contains(text, "CGO_ENABLED=0") {
		t.Fatal("scripts/release-build.sh disables CGO; release artifacts must keep the Apex parser available")
	}
	if !strings.Contains(text, "doctor --json") || !strings.Contains(text, `"parserOK": true`) {
		t.Fatal("scripts/release-build.sh must verify doctor --json reports parserOK true")
	}
	if strings.Contains(text, "doctor 2>/dev/null | grep -q") {
		t.Fatal("scripts/release-build.sh must not pipe doctor into grep -q under pipefail")
	}
}

func TestNoTrackedBuildArtifacts(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range repoCachedFiles(t, root) {
		if isTrackedBuildArtifact(rel) {
			t.Errorf("%s is a tracked build artifact", rel)
		}
	}
}

func TestRepoRootProjectScopeStaysNarrow(t *testing.T) {
	root := repoRoot(t)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}

	wantPackageDirs := []string{"testdata/local-tests/lwc-shell/force-app"}
	if got := packageDirPaths(p.PackageDirectories); !stringSlicesEqual(got, wantPackageDirs) {
		t.Fatalf("repo root packageDirs = %#v, want %#v", got, wantPackageDirs)
	}

	for _, file := range projectFiles(p) {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			t.Fatalf("rel %s: %v", file, err)
		}
		rel = filepath.ToSlash(rel)
		if isBroadRootProjectFile(rel) {
			t.Errorf("repo root project loaded out-of-scope file %s", rel)
		}
	}
}

func TestTrackedBuildArtifactMatcher(t *testing.T) {
	tests := map[string]bool{
		"apextest.test":              true,
		"internal/vm/vm.test":        true,
		"bin/glade":                  true,
		"dist/SHA256SUMS.txt":        true,
		"coverage.out":               true,
		"internal/server/.DS_Store":  true,
		"docs/coverage.out.md":       false,
		"internal/testreport/report": false,
	}
	for rel, want := range tests {
		if got := isTrackedBuildArtifact(rel); got != want {
			t.Fatalf("isTrackedBuildArtifact(%q) = %v, want %v", rel, got, want)
		}
	}
}

func packageDirPaths(dirs []project.PackageDirectory) []string {
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		out = append(out, filepath.ToSlash(dir.Path))
	}
	return out
}

func stringSlicesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func projectFiles(p project.Project) []string {
	var out []string
	out = append(out, p.ApexFiles...)
	out = append(out, p.ObjectFiles...)
	out = append(out, p.FieldFiles...)
	out = append(out, p.FieldSetFiles...)
	out = append(out, p.RecordTypeFiles...)
	out = append(out, p.ValidationRuleFiles...)
	out = append(out, p.LabelFiles...)
	out = append(out, p.TranslationFiles...)
	out = append(out, p.StaticResourceFiles...)
	out = append(out, p.StaticResourceMetas...)
	out = append(out, p.DataCategoryGroupFiles...)
	out = append(out, p.DataWeaveFiles...)
	out = append(out, p.DataWeaveMetas...)
	out = append(out, p.ContentAssetFiles...)
	out = append(out, p.ContentAssetMetas...)
	out = append(out, p.EmailTemplateFiles...)
	out = append(out, p.FolderFiles...)
	out = append(out, p.NamedCredentialFiles...)
	out = append(out, p.RemoteSiteFiles...)
	out = append(out, p.CustomMetadataFiles...)
	out = append(out, p.WorkflowFiles...)
	out = append(out, p.FlowFiles...)
	out = append(out, p.ProfileFiles...)
	out = append(out, p.PermissionSetFiles...)
	out = append(out, p.PermissionSetGroupFiles...)
	out = append(out, p.PermissionAssignmentFiles...)
	out = append(out, p.ListViewFiles...)
	out = append(out, p.LayoutFiles...)
	out = append(out, p.CompactLayoutFiles...)
	out = append(out, p.TabFiles...)
	out = append(out, p.WebLinkFiles...)
	out = append(out, p.QuickActionFiles...)
	out = append(out, p.GlobalValueSetFiles...)
	out = append(out, p.StandardValueSetFiles...)
	out = append(out, p.FlexiPageFiles...)
	out = append(out, p.ApplicationFiles...)
	out = append(out, p.VisualforcePageFiles...)
	out = append(out, p.VisualforceComponentFiles...)
	out = append(out, p.AuraFiles...)
	out = append(out, p.LWCFiles...)
	out = append(out, p.LWCHTMLFiles...)
	out = append(out, p.LWCCSSFiles...)
	out = append(out, p.LWCMetaFiles...)
	return out
}

func isBroadRootProjectFile(rel string) bool {
	if strings.HasPrefix(rel, hyphen("example", "projects")+"/") || strings.HasPrefix(rel, ".sfdx/") || strings.HasPrefix(rel, ".sf/") {
		return true
	}
	if strings.Contains(rel, "/testdata/") || strings.HasPrefix(rel, "internal/") {
		return true
	}
	return false
}

func TestAgentGuideListsCurrentProductCommands(t *testing.T) {
	root := repoRoot(t)
	guide := readRepoFile(t, root, "AGENTS.md")
	for _, command := range []string{
		"`version`", "`update`", "`doctor`", "`toolchain`", "`config`",
		"`init`", "`parse`", "`inspect`", "`schema`", "`refactor`",
		"`check`", "`exec`", "`debug`", "`editor`", "`dap`", "`test`",
		"`tui`", "`dev`", "`report`", "`lsp`", "`profile`", "`examples`",
		"`explain`", "`support`", "`plugins`", "`package`", "`server`",
		"`org`", "`playground`", "`db`", "`completion`", "`help`",
	} {
		if !strings.Contains(guide, command) {
			t.Fatalf("AGENTS.md product command list is missing %s", command)
		}
	}
}

func regularFileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func TestGenericSObjectHandlersAvoidStandardObjectShortcuts(t *testing.T) {
	root := repoRoot(t)
	paths := []string{
		"internal/vm/platform_sobject_members.go",
		"internal/vm/sobject_member_dispatch.go",
		"internal/vm/lookup_assign.go",
	}
	terms := []string{"Opportunity", "OpportunityStage", "StageName", "IsClosed", "IsWon"}
	for _, rel := range paths {
		text := readRepoFile(t, root, rel)
		for _, term := range terms {
			if strings.Contains(text, term) {
				t.Errorf("%s contains generic SObject shortcut term %q", rel, term)
			}
		}
	}
}

func TestPluginMaintenanceBoundary(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"internal/apexdocs",
		"internal/" + "perfscan",
		"internal/surfaceledger",
		"internal/projectscan",
		"internal/compat",
		"docs/fixtures",
	} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			t.Errorf("%s must live in a plugin, not the product repo", rel)
		} else if !os.IsNotExist(err) {
			t.Errorf("stat %s: %v", rel, err)
		}
	}

	for _, rel := range repoGoFiles(t, root) {
		text := readRepoFile(t, root, rel)
		toolsImport := regexp.MustCompile(`(?m)^\s*(?:\w+\s+)?"github\.com/glade-sh/glade/` + "tools")
		if toolsImport.MatchString(text) {
			t.Errorf("%s imports glade-tools; product repo must not depend on plugins", rel)
		}
		perfscanImport := regexp.MustCompile(`(?m)^\s*(?:\w+\s+)?"github\.com/glade-sh/glade/internal/` + "perfscan")
		if perfscanImport.MatchString(text) {
			t.Errorf("%s imports %s; performance scan belongs to the plugin", rel, "internal/"+"perfscan")
		}
	}

	help := readRepoFile(t, root, "internal/cliui/help.go")
	if strings.Contains(help, "inspect performance") {
		t.Error("product help still lists inspect performance")
	}

	for _, rel := range []string{"README.md", "docs/LOCAL_TESTING.md", "docs/INSTALL.md", "docs/COMPATIBILITY.md"} {
		if strings.Contains(readRepoFile(t, root, rel), "glade inspect performance") {
			t.Errorf("%s still routes performance scans through glade inspect performance", rel)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return canonicalRepoRoot
}

func repositoryRootPath() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("runtime caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func repoGoFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	for _, dir := range []string{"cmd", "internal"} {
		base := filepath.Join(root, dir)
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return files
}

func repoTrackedFiles(t *testing.T, root string) []string {
	t.Helper()
	return repoCachedFiles(t, root)
}

func repoCachedFiles(t *testing.T, root string) []string {
	t.Helper()
	snapshot, err := repoScan(root)
	if err != nil {
		t.Fatal(err)
	}
	return append([]string(nil), snapshot.paths...)
}

func repoRegularFiles(t *testing.T, root string) []repoScanFile {
	t.Helper()
	snapshot, err := repoScan(root)
	if err != nil {
		t.Fatal(err)
	}
	return append([]repoScanFile(nil), snapshot.files...)
}

func repoExistingTrackedPaths(t *testing.T, root string) []string {
	t.Helper()
	files := repoRegularFiles(t, root)
	paths := make([]string, 0, len(files))
	for _, file := range files {
		paths = append(paths, file.rel)
	}
	return paths
}

func repoScan(root string) (repoScanSnapshot, error) {
	if filepath.Clean(root) == canonicalRepoRoot {
		return canonicalRepoScan.snapshot()
	}
	return buildRepoScan(root)
}

func buildRepoScan(root string) (repoScanSnapshot, error) {
	return repoScanBuilder{
		inventory: repoCachedFileInventory,
		stat:      os.Stat,
		readFile:  os.ReadFile,
	}.scan(root)
}

func (builder repoScanBuilder) scan(root string) (repoScanSnapshot, error) {
	files, err := builder.inventory(root)
	if err != nil {
		return repoScanSnapshot{}, err
	}
	snapshot := repoScanSnapshot{
		paths: append([]string(nil), files...),
		files: make([]repoScanFile, 0, len(files)),
		byRel: make(map[string]repoScanFile, len(files)),
	}
	for _, rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, err := builder.stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return repoScanSnapshot{}, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		data, err := builder.readFile(path)
		if err != nil {
			return repoScanSnapshot{}, err
		}
		file := repoScanFile{rel: rel, text: string(data)}
		snapshot.files = append(snapshot.files, file)
		snapshot.byRel[rel] = file
	}
	return snapshot, nil
}

func repoCachedFileInventory(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "--cached")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files --cached: %w", err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		files = append(files, filepath.ToSlash(line))
	}
	return files, nil
}

func isTrackedBuildArtifact(rel string) bool {
	path := filepath.ToSlash(rel)
	base := filepath.Base(path)
	return strings.HasSuffix(base, ".test") ||
		base == ".DS_Store" ||
		base == "coverage.out" ||
		strings.HasPrefix(path, "bin/") ||
		strings.HasPrefix(path, "dist/")
}

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	snapshot, err := repoScan(root)
	if err != nil {
		t.Fatal(err)
	}
	if file, ok := snapshot.byRel[filepath.ToSlash(rel)]; ok {
		return file.text
	}
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func isHumanFacingExampleSurface(rel string) bool {
	return rel == "README.md" ||
		rel == "AGENTS.md" ||
		rel == "internal/cliui/help.go" ||
		rel == "internal/playground/examples.go" ||
		strings.HasPrefix(rel, "docs/") ||
		strings.HasPrefix(rel, "site/docs-src/")
}

func genericExampleNameTerms() []string {
	return []string{
		"example-project",
		"help-fixture",
		"my-project",
		"my-glade-org",
		"local-org.sqlite",
		"account-service",
		"AccountService",
		"InvoiceService",
		"BillingService",
	}
}

func allowsCorpusTerms(rel string) bool {
	return false
}

func forbiddenCorpusTerms() []string {
	return []string{
		hyphen("example", "projects"),
		"np" + "sp",
		strings.ToLower("NU" + "Exception"),
		hyphen("sf", "cred"),
		hyphen("src", "nmb"),
		hyphen("nams", "workspace"),
		hyphen("apex", "recipes"),
		"nu" + "tpl",
		dot("np"+"sp", "json"),
		dot("nu", "json"),
		dot(hyphen("sf", "cred"), "json"),
		dot("recipes", "json"),
		dot("nams", "json"),
		dot("nc", "json"),
	}
}

func hyphen(parts ...string) string {
	return strings.Join(parts, "-")
}

func dot(parts ...string) string {
	return strings.Join(parts, ".")
}

type privatePackageReporter interface {
	Helper()
	Errorf(string, ...any)
}

func checkPrivateExamplePackageText(t privatePackageReporter, rel, text string) {
	t.Helper()
	checkPrivateNameText(t, privateExamplePackagePatternSet, rel, text)
}

func checkPrivateNameText(t privatePackageReporter, patterns []privatePackagePattern, rel, text string) {
	t.Helper()
	for index, matched := range matchPrivateNames(patterns, text, true) {
		if matched {
			t.Errorf("%s contains private example package marker %q", rel, patterns[index].label)
		}
	}
}

func privateExamplePackageFinding(text string) string {
	return privateNameFinding(privateExamplePackagePatternSet, text)
}

func privateNameFinding(patterns []privatePackagePattern, text string) string {
	for index, matched := range matchPrivateNames(patterns, text, true) {
		if matched {
			return patterns[index].label
		}
	}
	return ""
}

var privateExamplePackagePatternSet = buildPrivateExamplePackagePatterns()

func privateExamplePackagePatterns() []privatePackagePattern {
	return append([]privatePackagePattern(nil), privateExamplePackagePatternSet...)
}

// Private example names are kept only as salted SHA-256 digests of their
// lower-case form, so this guard finds them without spelling them out. A
// pattern's kind fixes the syntax around its name; matchPrivateNames finds the
// spans of a text that the kind allows and compares their digests.
const privateNameSalt = "glade repoguard private name v1\x00"

type privateNameKind uint8

const (
	// privateNameHyphenated is (?i)NAME anywhere; NAME is letters joined by
	// single hyphens.
	privateNameHyphenated privateNameKind = iota
	// privateNameWord is (?i)\bNAME\b.
	privateNameWord
	// privateNameWordPrefix is (?i)\bNAME[[:alnum:]_]*\b.
	privateNameWordPrefix
	// privateNamePackage is (?i)\bNAME(?:__|\b|\.).
	privateNamePackage
	// privateNameProduct is (?i)\bNAME\b|NAME__|NAME[:/-]|\bNAME[A-Z_].
	privateNameProduct
	// privateNameNamespace is NAME, written in upper case, as a namespace:
	// (?i:\bNAME__|\bzNAME__|namespace"\s*:\s*"NAME"|Namespace:\s*"NAME"|
	// Namespace\s*=\s*"NAME"|SetCurrentNamespace\("NAME"\)|Type\.forName\('NAME')
	// or the case-sensitive \bNAME\.[A-Za-z].
	privateNameNamespace
	// privateNamePhrase is (?i)\bNAME\b; NAME is letters separated by spaces.
	privateNamePhrase
)

// privateNameTable lists the guarded names in finding order: the kind, the
// name's length in runes, privateNameFingerprint of the lower-case name (a
// cheap prefilter), and the hex SHA-256 of privateNameSalt and that name.
// Names are ASCII letters, with hyphens for privateNameHyphenated and
// spaces for privateNamePhrase.
var privateNameTable = []struct {
	kind        privateNameKind
	length      int
	fingerprint uint16
	digest      string
}{
	{privateNameHyphenated, 22, 0xa826, "efbeaaec446d00569c25d1f611cb6a9b8523e34f6dff0527e07d1d71792d13da"},
	{privateNameHyphenated, 25, 0xee64, "d740ea594fcda17f1c029e873c3ddad6f430626866089dcc7b1fead392f903f4"},
	{privateNameHyphenated, 18, 0x8e86, "a4fbdfbe205df67091c6a474b71d0f86b6c6c2e61fc36d036c6f24476cee73c3"},
	{privateNameHyphenated, 18, 0xf977, "55786f086c337ee2e2fcabc7ea2ebff2f06ad00e0cd5bd04bb41de5f0f35c0ab"},
	{privateNameHyphenated, 21, 0xd914, "b9400938d1ef900ef637125fde04a4350007f64a3430bd1159b390dba72149f2"},
	{privateNameHyphenated, 19, 0x7e04, "61c31d07aeab74d1a63af83db788aea64c35e24ecd1391be66a15559d1707feb"},
	{privateNameHyphenated, 21, 0x41d4, "3e051b48b277ebe78922fd4036e9391716886ee9a5ed58b822f205df434ad825"},
	{privateNameHyphenated, 21, 0xde2e, "e1d94ad5a58426fb87da4633109865af4815ac8ee4b85f8307b46a1997b8e974"},
	{privateNameHyphenated, 19, 0x49be, "976ae88a565535a1321b57f3dc5b513b4ad328622ffe9830d4d51541f4f84067"},
	{privateNamePackage, 4, 0x405e, "d6f4cf24eb84ac8f7d7e6d6a4f3182b3704a23ed8b57856ffa05a43da305e1d2"},
	{privateNameNamespace, 2, 0x0f3a, "9cfd1f2d31418a841032bc365dcfb243a351fb245a35854cdfc1b423520eecb6"},
	{privateNameProduct, 10, 0xf109, "b65da6f09c152e8f69bca11c2ab2d15308c064af16048bfaaef98cec6137af5d"},
	{privateNameWordPrefix, 6, 0x0948, "da68d32409c8384360ce2f00685673e58aa946de54b892c62d5082c60a4d4c28"},
	{privateNameWord, 9, 0x6581, "805cef92237b3652b373adf53c284dd57a7ea51fed398a3cb8527632e8f2ea7f"},
	{privateNameHyphenated, 7, 0xc9bb, "f58ebba1ad45c628dc5ef48064a3f2ed210c93851521d1a4e03e33e6df402f0c"},
	{privateNameWordPrefix, 13, 0x4c09, "1cbeb634e4e8c9d1874a40e4ff083347909dfdff62294ff97b7fd6ff6ff6416c"},
	{privateNameWordPrefix, 22, 0xa9a7, "483719d7ce249d9a8a92f0d356d02583023c13589085e11517cf5ea8ff57a10a"},
	{privateNameWordPrefix, 11, 0x36aa, "8ab70b2b4c792a336e785c569cbae2bfa5d19adab2913ca68dd8e282621145ce"},
	{privateNameWordPrefix, 10, 0x2ea4, "2f915681f283abe9cd72a23519503a49549cf4ee751c025e21cb416dbfa6d4ce"},
	{privateNameWordPrefix, 13, 0x48b5, "738f947092646c23753821093b0a711ea47ee27ae49d5971921e0fc75cfa08dc"},
	{privateNameWordPrefix, 16, 0x6dc9, "308f1ecef2ee1fb50883c7959fbb1660ecee86142308e1416b5955486ab29903"},
	{privateNameWordPrefix, 24, 0xb497, "8e78b1c099d33b82380e7ac37750710e416b4a0a7df3b159261db78488eda5e7"},
	{privateNamePhrase, 15, 0x88a4, "dd538701916e4588093d6b1ac427561ad5250bc03ef812cb0940f99c66526414"},
	{privateNamePhrase, 30, 0x1011, "0063f9509f624b3f44c98edf0ec395238beaf73b2517d11b45486e151f46bcc7"},
}

type privatePackagePattern struct {
	label       string
	kind        privateNameKind
	length      int
	fingerprint uint16
	digest      [sha256.Size]byte
}

func buildPrivateExamplePackagePatterns() []privatePackagePattern {
	patterns := make([]privatePackagePattern, len(privateNameTable))
	for index, row := range privateNameTable {
		digest, err := hex.DecodeString(row.digest)
		if err != nil || len(digest) != sha256.Size || row.length <= 0 {
			panic(fmt.Sprintf("private name table row %d is malformed", index+1))
		}
		patterns[index] = privatePackagePattern{
			label:       fmt.Sprintf("private-pattern-%02d", index+1),
			kind:        row.kind,
			length:      row.length,
			fingerprint: row.fingerprint,
		}
		copy(patterns[index].digest[:], digest)
	}
	return patterns
}

// matches reports whether text contains the pattern's name.
func (pattern privatePackagePattern) matches(text string) bool {
	return matchPrivateNames([]privatePackagePattern{pattern}, text, true)[0]
}

func privateNameSum(name []byte) [sha256.Size]byte {
	var buffer [96]byte
	return sha256.Sum256(append(append(buffer[:0], privateNameSalt...), name...))
}

const (
	privateFNVOffset uint32 = 2166136261
	privateFNVPrime  uint32 = 16777619
)

// privateNameFingerprint is FNV-1a over the lower-case name, folded to 16 bits.
func privateNameFingerprint(name []byte) uint16 {
	h := privateFNVOffset
	for _, c := range name {
		h = privateFNVStep(h, c)
	}
	return privateFNVFold(h)
}

func privateFNVStep(h uint32, c byte) uint32 {
	return (h ^ uint32(c)) * privateFNVPrime
}

func privateFNVFold(h uint32) uint16 {
	return uint16(h ^ h>>16)
}

// A scanned text keeps one byte per rune: the rune when ASCII, or one of these.
const (
	privateRuneOther  = 0x80
	privateRuneKelvin = 0x81 // U+212A KELVIN SIGN, which (?i) matches with k
	privateRuneLongS  = 0x82 // U+017F LATIN SMALL LETTER LONG S, which (?i) matches with s
)

// privateRuneFold maps a scanned byte to the lower-case letter that (?i)
// matches it with, or to itself. privateRuneWord marks the bytes that \b
// treats as word characters.
var privateRuneFold, privateRuneWord = buildPrivateRuneTables()

func buildPrivateRuneTables() (fold [256]byte, word [256]bool) {
	for c := range fold {
		fold[c] = byte(c)
		word[c] = c < utf8.RuneSelf && syntax.IsWordChar(rune(c))
	}
	for c := 'A'; c <= 'Z'; c++ {
		fold[c] = byte(c - 'A' + 'a')
	}
	fold[privateRuneKelvin], fold[privateRuneLongS] = 'k', 's'
	return fold, word
}

func privateRuneByte(r rune) byte {
	switch {
	case r < utf8.RuneSelf:
		return byte(r)
	case r == '\u212a':
		return privateRuneKelvin
	case r == '\u017f':
		return privateRuneLongS
	}
	return privateRuneOther
}

// privateNameRunes returns text with each rune replaced by privateRuneByte.
func privateNameRunes(text string) string {
	for index := 0; index < len(text); index++ {
		if text[index] >= utf8.RuneSelf {
			runes := make([]byte, 0, len(text))
			for _, r := range text {
				runes = append(runes, privateRuneByte(r))
			}
			return string(runes)
		}
	}
	return text
}

func privateLetter(c byte) bool {
	return 'a' <= c && c <= 'z'
}

type privateNameScan struct {
	text      string // the scanned text
	runes     string // privateNameRunes(text): rune i is byte i
	patterns  []privatePackagePattern
	prefilter bool // false hashes every candidate span, ignoring fingerprints
	matched   []bool
	contexts  bool   // a letter run spells namespace or forname
	name      []byte // the folded candidate span
}

// matchPrivateNames reports, for each pattern, whether text contains its name
// in the syntax its kind allows. With prefilter set, the 16-bit fingerprint
// rejects most candidate spans before their SHA-256 is computed.
func matchPrivateNames(patterns []privatePackagePattern, text string, prefilter bool) []bool {
	scan := privateNameScan{
		text:      text,
		runes:     privateNameRunes(text),
		patterns:  patterns,
		prefilter: prefilter,
		matched:   make([]bool, len(patterns)),
	}
	namespace := privateNameOrder(patterns, privateNameNamespace)
	scan.scanLetterRuns(
		privateNameOrder(patterns, privateNameWord, privateNameWordPrefix, privateNamePackage, privateNameProduct, privateNameNamespace),
		privateNameOrder(patterns, privateNameProduct),
		namespace)
	scan.scanHyphenatedRuns(privateNameOrder(patterns, privateNameHyphenated))
	scan.scanPhraseSpans(privateNameOrder(patterns, privateNamePhrase))
	if scan.contexts {
		scan.scanNamespaceContexts(namespace)
	}
	return scan.matched
}

// privateNameOrder returns the indexes of the patterns of the given kinds,
// shortest name first.
func privateNameOrder(patterns []privatePackagePattern, kinds ...privateNameKind) []int {
	var order []int
	for index, pattern := range patterns {
		if slices.Contains(kinds, pattern.kind) {
			order = append(order, index)
		}
	}
	sort.SliceStable(order, func(left, right int) bool {
		return patterns[order[left]].length < patterns[order[right]].length
	})
	return order
}

func (scan *privateNameScan) fold(index int) byte {
	return privateRuneFold[scan.raw(index)]
}

// raw returns scanned byte index, or 0 outside the text.
func (scan *privateNameScan) raw(index int) byte {
	if index < 0 || index >= len(scan.runes) {
		return 0
	}
	return scan.runes[index]
}

// boundary reports whether \b holds before rune index.
func (scan *privateNameScan) boundary(index int) bool {
	return privateRuneWord[scan.raw(index-1)] != privateRuneWord[scan.raw(index)]
}

func (scan *privateNameScan) underscores(index int) bool {
	return scan.raw(index) == '_' && scan.raw(index+1) == '_'
}

func (scan *privateNameScan) upper(start, end int) bool {
	for index := start; index < end; index++ {
		if c := scan.raw(index); c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}

func (scan *privateNameScan) spanFNV(start, end int) uint32 {
	h := privateFNVOffset
	for index := start; index < end; index++ {
		h = privateFNVStep(h, scan.fold(index))
	}
	return h
}

// spells reports whether the folded span [start, end) contains word.
func (scan *privateNameScan) spells(start, end int, word string) bool {
	for at := start; at+len(word) <= end; at++ {
		index := 0
		for index < len(word) && scan.fold(at+index) == word[index] {
			index++
		}
		if index == len(word) {
			return true
		}
	}
	return false
}

// nameAt reports whether the folded span of the pattern's length at start is
// its name; h is FNV-1a state over that span.
func (scan *privateNameScan) nameAt(pattern *privatePackagePattern, start int, h uint32) bool {
	if scan.prefilter && privateFNVFold(h) != pattern.fingerprint {
		return false
	}
	scan.name = scan.name[:0]
	for index := start; index < start+pattern.length; index++ {
		scan.name = append(scan.name, scan.fold(index))
	}
	return privateNameSum(scan.name) == pattern.digest
}

// scanLetterRuns visits each maximal run of folded letters. Names other than
// hyphenated names and phrases lie inside one run.
func (scan *privateNameScan) scanLetterRuns(wordStart, product, namespace []int) {
	n := len(scan.runes)
	for start := 0; start < n; start++ {
		if !privateLetter(scan.fold(start)) {
			continue
		}
		end := start + 1
		for end < n && privateLetter(scan.fold(end)) {
			end++
		}
		for at := start; at < end; at++ {
			if !scan.boundary(at) {
				continue
			}
			scan.tryNames(at, end, wordStart, true)
			if scan.fold(at) == 'z' {
				scan.afterZ(at+1, end, namespace)
			}
		}
		scan.atRunEnd(start, end, product)
		if len(namespace) > 0 && !scan.contexts {
			scan.contexts = scan.spells(start, end, "namespace") || scan.spells(start, end, "forname")
		}
		start = end
	}
}

// tryNames checks the names in order, shortest first, that start at start
// and end by runEnd. After a word boundary, completes checks the text that
// follows each name.
func (scan *privateNameScan) tryNames(start, runEnd int, order []int, afterBoundary bool) {
	h, end := privateFNVOffset, start
	for _, index := range order {
		pattern := &scan.patterns[index]
		if start+pattern.length > runEnd {
			return
		}
		for ; end < start+pattern.length; end++ {
			h = privateFNVStep(h, scan.fold(end))
		}
		if !scan.matched[index] && (!afterBoundary || scan.completes(pattern, start, end)) && scan.nameAt(pattern, start, h) {
			scan.matched[index] = true
		}
	}
}

// completes reports whether the text after a name that starts at a word
// boundary completes the pattern.
func (scan *privateNameScan) completes(pattern *privatePackagePattern, start, end int) bool {
	next := scan.fold(end)
	switch pattern.kind {
	case privateNameWord:
		return scan.boundary(end)
	case privateNameWordPrefix:
		for ; !scan.boundary(end); end++ {
			if c := scan.fold(end); !privateLetter(c) && (c < '0' || c > '9') && c != '_' {
				return false
			}
		}
		return true
	case privateNamePackage:
		return scan.underscores(end) || scan.boundary(end) || next == '.'
	case privateNameProduct:
		// \bNAME\b, \bNAME[A-Z_] (which covers NAME__), and NAME[:/-].
		return scan.boundary(end) || privateLetter(next) || next == '_' || next == ':' || next == '/' || next == '-'
	case privateNameNamespace:
		after := scan.raw(end + 1)
		return scan.underscores(end) ||
			scan.upper(start, end) && scan.raw(end) == '.' && ('A' <= after && after <= 'Z' || 'a' <= after && after <= 'z')
	}
	return false
}

// afterZ checks \bzNAME__ for namespace names that start at start.
func (scan *privateNameScan) afterZ(start, runEnd int, namespace []int) {
	for _, index := range namespace {
		pattern := &scan.patterns[index]
		end := start + pattern.length
		if !scan.matched[index] && end <= runEnd && scan.underscores(end) && scan.nameAt(pattern, start, scan.spanFNV(start, end)) {
			scan.matched[index] = true
		}
	}
}

// atRunEnd checks NAME__ and NAME[:/-] for product names that end the run,
// which need no word boundary before them.
func (scan *privateNameScan) atRunEnd(runStart, runEnd int, product []int) {
	if next := scan.fold(runEnd); !scan.underscores(runEnd) && next != ':' && next != '/' && next != '-' {
		return
	}
	for _, index := range product {
		pattern := &scan.patterns[index]
		start := runEnd - pattern.length
		if !scan.matched[index] && start >= runStart && scan.nameAt(pattern, start, scan.spanFNV(start, runEnd)) {
			scan.matched[index] = true
		}
	}
}

// scanHyphenatedRuns visits each maximal run of folded letters and hyphens
// that holds a hyphen, and checks each hyphenated name at each letter in it.
func (scan *privateNameScan) scanHyphenatedRuns(order []int) {
	if len(order) == 0 {
		return
	}
	n := len(scan.runes)
	for start := 0; start < n; start++ {
		if c := scan.fold(start); !privateLetter(c) && c != '-' {
			continue
		}
		end, hyphen := start, false
		for ; end < n; end++ {
			c := scan.fold(end)
			if c == '-' {
				hyphen = true
			} else if !privateLetter(c) {
				break
			}
		}
		if hyphen {
			for at := start; at < end; at++ {
				if privateLetter(scan.fold(at)) {
					scan.tryNames(at, end, order, false)
				}
			}
		}
		start = end
	}
}

// scanPhraseSpans checks fixed-length phrases between word boundaries.
// The digest also checks the spaces, so unrelated words cannot match.
func (scan *privateNameScan) scanPhraseSpans(order []int) {
	if len(order) == 0 {
		return
	}
	for start := 0; start < len(scan.runes); start++ {
		if !privateLetter(scan.fold(start)) || !scan.boundary(start) {
			continue
		}
		for _, index := range order {
			pattern := &scan.patterns[index]
			end := start + pattern.length
			if end > len(scan.runes) {
				break
			}
			if !scan.matched[index] && scan.boundary(end) && scan.nameAt(pattern, start, scan.spanFNV(start, end)) {
				scan.matched[index] = true
			}
		}
	}
}

// privateNamespaceOpening validates a candidate starting with one of the
// public context prefixes, without scanning the rest of the file on failure.
var privateNamespaceOpening = regexp.MustCompile(`(?i)^(?:(namespace"\s*:\s*"|Namespace:\s*"|Namespace\s*=\s*")|(SetCurrentNamespace\(")|(Type\.forName\('))`)

func (scan *privateNameScan) scanNamespaceContexts(namespace []int) {
	for offset, r := range scan.text {
		prefix := ""
		switch privateRuneFold[privateRuneByte(r)] {
		case 'n':
			prefix = "namespace"
		case 's':
			prefix = "setcurrentnamespace"
		case 't':
			prefix = "type.forname"
		default:
			continue
		}
		if !privateContextPrefix(scan.text[offset:], prefix) {
			continue
		}
		opening := privateNamespaceOpening.FindStringSubmatchIndex(scan.text[offset:])
		if opening == nil {
			continue
		}
		closing := `"`
		if opening[4] >= 0 {
			closing = `")`
		} else if opening[6] >= 0 {
			closing = `'`
		}
		for _, index := range namespace {
			pattern := &scan.patterns[index]
			if !scan.matched[index] && scan.quotedName(pattern, offset+opening[1], closing) {
				scan.matched[index] = true
			}
		}
	}
}

func privateContextPrefix(text, prefix string) bool {
	for _, want := range prefix {
		r, size := utf8.DecodeRuneInString(text)
		if size == 0 || privateRuneFold[privateRuneByte(r)] != byte(want) {
			return false
		}
		text = text[size:]
	}
	return true
}

// quotedName reports whether the text at byte offset is the pattern's name
// followed by closing.
func (scan *privateNameScan) quotedName(pattern *privatePackagePattern, offset int, closing string) bool {
	scan.name = scan.name[:0]
	h := privateFNVOffset
	for len(scan.name) < pattern.length {
		r, size := utf8.DecodeRuneInString(scan.text[offset:])
		c := privateRuneFold[privateRuneByte(r)]
		if size == 0 || !privateLetter(c) {
			return false
		}
		scan.name = append(scan.name, c)
		h = privateFNVStep(h, c)
		offset += size
	}
	if !strings.HasPrefix(scan.text[offset:], closing) {
		return false
	}
	if scan.prefilter && privateFNVFold(h) != pattern.fingerprint {
		return false
	}
	return privateNameSum(scan.name) == pattern.digest
}

func TestPrivateExamplePackagePatternsReuseDecodedSet(t *testing.T) {
	first := privateExamplePackagePatterns()
	second := privateExamplePackagePatterns()
	if len(first) != 24 {
		t.Fatalf("private example package pattern count = %d, want 24", len(first))
	}
	if !slices.Equal(first, second) || !slices.Equal(first, buildPrivateExamplePackagePatterns()) {
		t.Fatal("private example package patterns differ between calls")
	}

	first[0].label = "mutated"
	if got := privateExamplePackagePatterns()[0].label; got == "mutated" {
		t.Fatal("private example package patterns share the cached slice")
	}
}

func TestPrivateExamplePackageFindingUsesThePatternSet(t *testing.T) {
	if got := privateExamplePackageFinding("public example package reference"); got != "" {
		t.Fatalf("finding for public text = %q, want none", got)
	}
	patterns := privateStandInPatterns()
	if got, want := privateNameFinding(patterns, "see Desk-Kit docs"), patterns[0].label; got != want {
		t.Fatalf("stand-in finding = %q, want %q", got, want)
	}
	if got := privateNameFinding(patterns, "public example package reference"); got != "" {
		t.Fatalf("stand-in finding for public text = %q, want none", got)
	}
}

func TestPrivateExamplePackagePatternChecksAreParallelSafe(t *testing.T) {
	const workers = 32
	patterns := privateStandInPatterns()
	marker := `Object.kits__Thing desk-kit Namespace: "KS"`
	var failures atomic.Int32
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for iteration := 0; iteration < 100; iteration++ {
				matched := matchPrivateNames(patterns, marker, true)
				if !matched[0] || !matched[5] || !matched[7] || privateExamplePackageFinding(marker) != "" {
					failures.Add(1)
				}
			}
		}()
	}
	group.Wait()
	if got := failures.Load(); got != 0 {
		t.Fatalf("parallel private pattern checks failed %d times", got)
	}
}

func TestRepoScanCacheSharesOneSnapshotAndError(t *testing.T) {
	var builds atomic.Int32
	cache := repoScanCache{build: func() (repoScanSnapshot, error) {
		builds.Add(1)
		return repoScanSnapshot{files: []repoScanFile{{rel: "one.txt", text: "one"}}}, nil
	}}

	const callers = 32
	var group sync.WaitGroup
	for caller := 0; caller < callers; caller++ {
		group.Add(1)
		go func() {
			defer group.Done()
			snapshot, err := cache.snapshot()
			if err != nil {
				t.Errorf("snapshot: %v", err)
				return
			}
			if len(snapshot.files) != 1 || snapshot.files[0].text != "one" {
				t.Errorf("snapshot = %#v, want one.txt", snapshot)
			}
		}()
	}
	group.Wait()
	if got := builds.Load(); got != 1 {
		t.Fatalf("snapshot builds = %d, want 1", got)
	}
}

func TestRepoScanCacheSharesBuildError(t *testing.T) {
	want := errors.New("inventory unavailable")
	var builds atomic.Int32
	cache := repoScanCache{build: func() (repoScanSnapshot, error) {
		builds.Add(1)
		return repoScanSnapshot{}, want
	}}
	for call := 0; call < 2; call++ {
		if _, err := cache.snapshot(); !errors.Is(err, want) {
			t.Fatalf("snapshot error = %v, want %v", err, want)
		}
	}
	if got := builds.Load(); got != 1 {
		t.Fatalf("error snapshot builds = %d, want 1", got)
	}
}

func TestRepoScanCacheReadsEachTrackedBodyOnce(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"one.txt", "two.txt"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var inventories atomic.Int32
	var reads atomic.Int32
	builder := repoScanBuilder{
		inventory: func(string) ([]string, error) {
			inventories.Add(1)
			return []string{"one.txt", "two.txt"}, nil
		},
		stat: os.Stat,
		readFile: func(path string) ([]byte, error) {
			reads.Add(1)
			return os.ReadFile(path)
		},
	}
	cache := repoScanCache{build: func() (repoScanSnapshot, error) {
		return builder.scan(root)
	}}
	for call := 0; call < 4; call++ {
		if _, err := cache.snapshot(); err != nil {
			t.Fatal(err)
		}
	}
	if got := inventories.Load(); got != 1 {
		t.Fatalf("inventories = %d, want 1", got)
	}
	if got := reads.Load(); got != 2 {
		t.Fatalf("body reads = %d, want 2", got)
	}
}

func TestRepoScanPreservesTrackedInventoryWithoutReadingDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o755); err != nil {
		t.Fatal(err)
	}
	snapshot, err := (repoScanBuilder{
		inventory: func(string) ([]string, error) { return []string{"directory"}, nil },
		stat:      os.Stat,
		readFile:  os.ReadFile,
	}).scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.paths) != 1 || snapshot.paths[0] != "directory" {
		t.Fatalf("tracked paths = %#v, want directory", snapshot.paths)
	}
	if len(snapshot.files) != 0 {
		t.Fatalf("regular files = %#v, want none", snapshot.files)
	}
}

func TestRepoScanDoesNotCacheTemporaryRepositoryRoots(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init temporary repository: %v\n%s", err, output)
	}
	path := filepath.Join(root, "one.txt")
	if err := os.WriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "add", "one.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add temporary repository: %v\n%s", err, output)
	}

	first, err := repoScan(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := repoScan(root)
	if err != nil {
		t.Fatal(err)
	}
	if first.files[0].text != "first" || second.files[0].text != "second" {
		t.Fatalf("temporary repository scans = %q then %q, want fresh reads", first.files[0].text, second.files[0].text)
	}
}

func TestRepoExistingTrackedPathsSkipsDeletedIndexEntries(t *testing.T) {
	root := t.TempDir()
	if output, err := exec.Command("git", "init", "--quiet", root).CombinedOutput(); err != nil {
		t.Fatalf("git init temporary repository: %v\n%s", err, output)
	}
	path := filepath.Join(root, "deleted.txt")
	if err := os.WriteFile(path, []byte("tracked"), 0o644); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command("git", "-C", root, "add", "deleted.txt").CombinedOutput(); err != nil {
		t.Fatalf("git add temporary repository: %v\n%s", err, output)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := repoExistingTrackedPaths(t, root); len(got) != 0 {
		t.Fatalf("existing tracked paths = %#v, want none", got)
	}
	if got := repoCachedFiles(t, root); len(got) != 1 || got[0] != "deleted.txt" {
		t.Fatalf("cached inventory = %#v, want deleted.txt", got)
	}
}
