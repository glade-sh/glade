package repoguard

import (
	"os"
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
	return rel == "internal/compat/server_examples.go" || rel == "internal/compat/server_examples_test.go"
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
