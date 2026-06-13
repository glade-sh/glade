package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

func TestSchemaImportDescribeWritesSchema(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "describe.json")
	output := filepath.Join(root, "schema.json")
	data := `{"objects":[{"name":"Account","label":"Account","labelPlural":"Accounts","fields":[{"name":"Id","type":"id","label":"Account ID","nillable":false},{"name":"Name","type":"string","label":"Account Name","nillable":false,"createable":true,"updateable":true}]}]}`
	if err := os.WriteFile(input, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"schema", "import", "describe", "--input", input, "--output", output}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	written, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), `"name": "Account"`) {
		t.Fatalf("schema output missing Account:\n%s", string(written))
	}
}

func TestRunTestSelectsExactClass(t *testing.T) {
	run := runSelectionTest(t, "--class", "AccountServiceTest")
	if got, want := classNames(run), []string{"AccountServiceTest"}; !equalStrings(got, want) {
		t.Fatalf("classes = %#v, want %#v", got, want)
	}
}

func TestRunTestSelectsExactMethod(t *testing.T) {
	run := runSelectionTest(t, "--class", "AccountServiceTest", "--method", "testCreatesAccount")
	if got, want := caseNames(run), []string{"AccountServiceTest.testCreatesAccount"}; !equalStrings(got, want) {
		t.Fatalf("cases = %#v, want %#v", got, want)
	}
}

func TestRunTestSelectsClassFile(t *testing.T) {
	root := selectionFixtureRoot(t)
	classFile := filepath.Join(t.TempDir(), "tests.txt")
	if err := os.WriteFile(classFile, []byte("# comment\nBillingServiceTest\n\nContactServiceTest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := runSelectionTestInRoot(t, root, "--class-file", classFile)
	if got, want := classNames(run), []string{"BillingServiceTest", "ContactServiceTest"}; !equalStrings(got, want) {
		t.Fatalf("classes = %#v, want %#v", got, want)
	}
}

func TestRunTestSelectsDeterministicClassShard(t *testing.T) {
	run := runSelectionTest(t, "--shard-count", "2", "--shard-index", "1")
	if got, want := classNames(run), []string{"AccountServiceTestExtra", "ContactServiceTest"}; !equalStrings(got, want) {
		t.Fatalf("classes = %#v, want %#v", got, want)
	}
}

func runSelectionTest(t *testing.T, args ...string) testreport.Run {
	t.Helper()
	return runSelectionTestInRoot(t, selectionFixtureRoot(t), args...)
}

func runSelectionTestInRoot(t *testing.T, root string, args ...string) testreport.Run {
	t.Helper()
	cliArgs := []string{"test", "--project", root, "--json", "--no-cache", "--no-progress"}
	cliArgs = append(cliArgs, args...)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), cliArgs, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	run, err := decodeTestRunJSON(stdout.Bytes())
	if err != nil {
		t.Fatalf("decode run: %v\n%s", err, stdout.String())
	}
	return run
}

func decodeTestRunJSON(data []byte) (testreport.Run, error) {
	var envelope struct {
		Data testreport.Run `json:"data"`
	}
	if err := json.Unmarshal(data, &envelope); err == nil && len(envelope.Data.Suites) > 0 {
		return envelope.Data, nil
	}
	var run testreport.Run
	err := json.Unmarshal(data, &run)
	return run, err
}

func selectionFixtureRoot(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "test-selection"))
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "test-selection")
	if err := copySelectionFixture(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func copySelectionFixture(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o755)
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

func classNames(run testreport.Run) []string {
	seen := map[string]bool{}
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.ClassName != "" {
				seen[testCase.ClassName] = true
			}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func caseNames(run testreport.Run) []string {
	var out []string
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			out = append(out, testCase.ClassName+"."+testCase.MethodName)
		}
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
