package repoguard

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

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

func checkPrivateExamplePackageText(t *testing.T, rel, text string) {
	t.Helper()
	for _, pattern := range privateExamplePackagePatternSet {
		if pattern.re.MatchString(text) {
			t.Errorf("%s contains private example package marker %q", rel, pattern.label)
		}
	}
}

func privateExamplePackageFinding(text string) string {
	for _, pattern := range privateExamplePackagePatternSet {
		if pattern.re.MatchString(text) {
			return pattern.label
		}
	}
	return ""
}

type privatePackagePattern struct {
	label string
	re    *regexp.Regexp
}

var privateExamplePackagePatternSet = buildPrivateExamplePackagePatterns()

func privateExamplePackagePatterns() []privatePackagePattern {
	return append([]privatePackagePattern(nil), privateExamplePackagePatternSet...)
}

func buildPrivateExamplePackagePatterns() []privatePackagePattern {
	ci := func(parts ...string) *regexp.Regexp {
		return regexp.MustCompile(`(?i)` + strings.Join(parts, ""))
	}
	cs := func(parts ...string) *regexp.Regexp {
		return regexp.MustCompile(strings.Join(parts, ""))
	}
	exact := func(parts ...string) *regexp.Regexp {
		return ci(`\b`, strings.Join(parts, ""), `\b`)
	}
	return []privatePackagePattern{
		{hyphen("src", "nbm", "solhub", "develop"), ci(hyphen("src", "nbm", "solhub", "develop"))},
		{hyphen("src", "nmb", strings.Join([]string{"nam", "z"}, ""), "prog", "develop"), ci(hyphen("src", "nmb", strings.Join([]string{"nam", "z"}, ""), "prog", "develop"))},
		{hyphen("src", "nmb", "nc", "develop"), ci(hyphen("src", "nmb", "nc", "develop"))},
		{hyphen("src", "nmb", "nu", "develop"), ci(hyphen("src", "nmb", "nu", "develop"))},
		{hyphen("src", "nmb", strings.Join([]string{"nu", "dev"}, ""), "develop"), ci(hyphen("src", "nmb", strings.Join([]string{"nu", "dev"}, ""), "develop"))},
		{hyphen("src", "nmb", strings.Join([]string{"nu", "q"}, ""), "develop"), ci(hyphen("src", "nmb", strings.Join([]string{"nu", "q"}, ""), "develop"))},
		{hyphen("src", "nmb", strings.Join([]string{"nu", "tpl"}, ""), "develop"), ci(hyphen("src", "nmb", strings.Join([]string{"nu", "tpl"}, ""), "develop"))},
		{hyphen("src", "nmb", strings.Join([]string{"nu", "tplx"}, ""), "master"), ci(hyphen("src", "nmb", strings.Join([]string{"nu", "tplx"}, ""), "master"))},
		{hyphen("sf", "cred", "pkg", "develop"), ci(hyphen("sf", "cred", "pkg", "develop"))},
		{strings.Join([]string{"nam", "z"}, ""), ci(`\bnam`, "z", `(?:__|\b|\.)`)},
		{strings.Join([]string{"N", "U namespace"}, ""), cs(`(?i:\bN`, "U", `__|\bn`, "u", `__|\bz`, "n", "u", `__|namespace"\s*:\s*"N`, "U", `"|Namespace:\s*"N`, "U", `"|Namespace\s*=\s*"N`, "U", `"|SetCurrentNamespace\("N`, "U", `"\)|Type\.forName\('N`, "U", `')|\bN`, "U", `\.[A-Za-z]`)},
		{strings.Join([]string{"veri", "fiable"}, ""), ci(`\b`, "veri", "fiable", `\b|`, "veri", "fiable", `__|`, "veri", "fiable", `[:/-]|\b`, "Veri", "fiable", `[A-Z_]`)},
		{strings.Join([]string{"nim", "ble"}, ""), ci(`\bnim`, "ble", `[[:alnum:]_]*\b`)},
		{strings.Join([]string{"mo", "mentive"}, ""), exact("mo", "mentive")},
		{hyphen("sf", "cred"), ci(hyphen("sf", "cred"))},
		{strings.Join([]string{"creden", "tialing"}, ""), ci(`\bcreden`, "tialing", `[[:alnum:]_]*\b`)},
		{strings.Join([]string{"V", "fiHospitalAffiliation"}, ""), ci(`\bV`, "fiHospitalAffiliation", `[[:alnum:]_]*\b`)},
		{strings.Join([]string{"V", "fiProvider"}, ""), ci(`\bV`, "fiProvider", `[[:alnum:]_]*\b`)},
		{strings.Join([]string{"V", "fiLicense"}, ""), ci(`\bV`, "fiLicense", `[[:alnum:]_]*\b`)},
		{strings.Join([]string{"Education", "Last"}, ""), ci(`\bEducation`, "Last", `[[:alnum:]_]*\b`)},
		{strings.Join([]string{"Setup", "DataMapping"}, ""), ci(`\bSetup`, "DataMapping", `[[:alnum:]_]*\b`)},
		{strings.Join([]string{"Action", "ProcessorQueueable"}, ""), ci(`\bAction`, "ProcessorQueueable", `[[:alnum:]_]*\b`)},
	}
}

func TestPrivateExamplePackagePatternsReuseCompiledExpressions(t *testing.T) {
	first := privateExamplePackagePatterns()
	second := privateExamplePackagePatterns()
	if len(first) != 22 {
		t.Fatalf("private pattern count = %d, want 22", len(first))
	}
	if len(first) != len(second) {
		t.Fatalf("private pattern lengths differ: %d and %d", len(first), len(second))
	}
	for index := range first {
		if first[index].label != second[index].label {
			t.Fatalf("pattern %d label differs: %q and %q", index, first[index].label, second[index].label)
		}
		if first[index].re != second[index].re {
			t.Fatalf("pattern %q was compiled more than once", first[index].label)
		}
	}

	first[0].label = "mutated"
	if got := privateExamplePackagePatterns()[0].label; got == "mutated" {
		t.Fatal("private pattern caller mutated the shared pattern slice")
	}
}

func TestPrivateExamplePackagePatternsMatchExistingFixtureCases(t *testing.T) {
	patterns := privateExamplePackagePatterns()
	tests := []struct {
		name string
		text string
		want string
	}{
		{name: "path marker", text: patterns[0].label, want: patterns[0].label},
		{name: "namespace marker", text: "Object." + patterns[9].label + "__Thing", want: patterns[9].label},
		{name: "negative", text: "macrodata-apex refinement-local", want: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ""
			for _, pattern := range privateExamplePackagePatterns() {
				if pattern.re.MatchString(test.text) {
					got = pattern.label
					break
				}
			}
			if got != test.want {
				t.Fatalf("pattern match for %q = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestPrivateExamplePackageFindingUsesTheCompiledPatternSet(t *testing.T) {
	marker := privateExamplePackagePatterns()[0].label
	if got, want := privateExamplePackageFinding(marker), marker; got != want {
		t.Fatalf("private marker finding = %q, want %q", got, want)
	}
	if got := privateExamplePackageFinding("macrodata-apex refinement-local"); got != "" {
		t.Fatalf("safe text finding = %q, want empty", got)
	}
}

func TestPrivateExamplePackagePatternChecksAreParallelSafe(t *testing.T) {
	const workers = 32
	marker := privateExamplePackagePatterns()[0].label
	var failures atomic.Int32
	var group sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for iteration := 0; iteration < 100; iteration++ {
				matched := false
				for _, pattern := range privateExamplePackagePatterns() {
					if pattern.re.MatchString(marker) {
						matched = true
						break
					}
				}
				if !matched {
					failures.Add(1)
				}
			}
		}()
	}
	group.Wait()
	if got := failures.Load(); got != 0 {
		t.Fatalf("parallel pattern checks lost %d matches", got)
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
