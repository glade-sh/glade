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
	cmd := exec.Command(node, filepath.Join(root, "scripts", "generate-system-stub-symbols.mjs"), inputRoot, output)
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

func readRepoFile(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
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
