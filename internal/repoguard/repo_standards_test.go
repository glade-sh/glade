package repoguard

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

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
	for _, rel := range repoTrackedFiles(t, root) {
		path := filepath.ToSlash(rel)
		fullPath := filepath.Join(root, filepath.FromSlash(rel))
		if _, err := os.Stat(fullPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatal(err)
		}
		checkPrivateExamplePackageText(t, path, path)

		fileData, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.IndexByte(fileData, 0) >= 0 {
			continue
		}
		checkPrivateExamplePackageText(t, path, string(fileData))
	}
}

func TestPublicExampleNamesAvoidGenericPlaceholders(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range repoTrackedFiles(t, root) {
		if !isHumanFacingExampleSurface(rel) {
			continue
		}
		fullPath := filepath.Join(root, filepath.FromSlash(rel))
		fileData, err := os.ReadFile(fullPath)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.IndexByte(fileData, 0) >= 0 {
			continue
		}
		text := string(fileData)
		for _, term := range genericExampleNameTerms() {
			if strings.Contains(text, term) {
				t.Errorf("%s contains generic example name %q; use macrodata-apex, RefinementService, FileRow, or refinement-local instead", rel, term)
			}
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

func TestGeneratedSystemStubsReproduceFromGenerator(t *testing.T) {
	root := repoRoot(t)
	inputRoot := filepath.Join(root, hyphen("example", "projects"), "stubs", "apex-system-stubs")
	if _, err := os.Stat(inputRoot); err != nil {
		t.Skipf("system stub input unavailable: %v", err)
	}
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skipf("node unavailable: %v", err)
	}
	output := filepath.Join(t.TempDir(), "system_stub_symbols_generated.go")
	args := []string{filepath.Join(root, "scripts", "generate-system-stub-symbols.mjs"), inputRoot, output}
	if contracts := filepath.Join(root, "testdata", "generated", "apex_docs_contracts.json"); regularFileExists(contracts) {
		args = append(args, contracts)
	}
	cmd := exec.Command(node, args...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate-system-stub-symbols failed: %v\n%s", err, out)
	}
	generated, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	checkedIn, err := os.ReadFile(filepath.Join(root, "internal", "typesys", "system_stub_symbols_generated.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(generated, checkedIn) {
		t.Fatal("internal/typesys/system_stub_symbols_generated.go does not match scripts/generate-system-stub-symbols.mjs output")
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
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
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
	cmd := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		files = append(files, filepath.ToSlash(line))
	}
	return files
}

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
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
	for _, pattern := range privateExamplePackagePatterns() {
		if pattern.re.MatchString(text) {
			t.Errorf("%s contains private example package marker %q", rel, pattern.label)
		}
	}
}

type privatePackagePattern struct {
	label string
	re    *regexp.Regexp
}

func privateExamplePackagePatterns() []privatePackagePattern {
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
