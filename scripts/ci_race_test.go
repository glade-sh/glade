package scripts

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func runRaceClassifier(t *testing.T, env []string, args ...string) ([]string, string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"ci-race-packages.sh"}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, stderr.String(), err
	}
	var packages []string
	if err := json.Unmarshal(stdout.Bytes(), &packages); err != nil {
		t.Fatalf("classifier returned invalid JSON: %v\nstdout=%s\nstderr=%s", err, stdout.Bytes(), stderr.Bytes())
	}
	return packages, stderr.String(), nil
}

func TestCIRaceClassifierHighRiskSet(t *testing.T) {
	got, out, err := runRaceClassifier(t, nil, "high-risk")
	if err != nil {
		t.Fatalf("high-risk classification failed: %v\n%s", err, out)
	}
	want := []string{
		"./internal/apextest", "./internal/gladecli", "./internal/playground",
		"./internal/sema", "./internal/server", "./internal/startupcache", "./internal/storage",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("high-risk packages = %v, want %v", got, want)
	}
}

func TestCIRaceClassifierFullMatchesManifest(t *testing.T) {
	got, out, err := runRaceClassifier(t, nil, "full")
	if err != nil {
		t.Fatalf("full classification failed: %v\n%s", err, out)
	}
	manifestData, err := os.ReadFile("ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Lanes map[string][]string `json:"lanes"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, packages := range manifest.Lanes {
		for _, pkg := range packages {
			want = append(want, "."+strings.TrimPrefix(pkg, "github.com/glade-sh/glade"))
		}
	}
	sort.Strings(want)
	if len(got) != 61 || !reflect.DeepEqual(got, want) {
		t.Fatalf("full packages = %d/%v, want 61 exact manifest packages", len(got), got)
	}
}

func TestCIRaceClassifierChangedIncludesDependents(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	goPath := filepath.Join(dir, "go")
	gitScript := "#!/usr/bin/env bash\nprintf 'M\\t%s\\n' internal/storage/store.go\n"
	goScript := `#!/usr/bin/env bash
case "$*" in
  *"./internal/storage") printf '%s\n' github.com/glade-sh/glade/internal/storage ;;
  *"./...")
    if [[ "$*" == *TestImports* ]]; then
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/storage '' '' ''
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/vm 'github.com/glade-sh/glade/internal/storage' '' ''
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/soql 'github.com/glade-sh/glade/internal/storage github.com/glade-sh/glade/internal/vm' '' ''
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/apexlog '' 'github.com/glade-sh/glade/internal/storage' ''
      printf '%s\t%s\t%s\t%s\n' github.com/glade-sh/glade/internal/cliui '' '' ''
    else
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/storage ''
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/vm 'github.com/glade-sh/glade/internal/storage'
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/soql 'github.com/glade-sh/glade/internal/storage github.com/glade-sh/glade/internal/vm'
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/apexlog ''
      printf '%s\t%s\n' github.com/glade-sh/glade/internal/cliui ''
    fi
    ;;
  *) exit 97 ;;
esac
`
	for path, contents := range map[string]string{gitPath: gitScript, goPath: goScript} {
		if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	got, out, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath, "CI_RACE_GO_COMMAND=" + goPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("changed classification failed: %v\n%s", err, out)
	}
	want := []string{"./internal/apexlog", "./internal/soql", "./internal/storage", "./internal/vm"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changed packages = %v, want %v", got, want)
	}
}

func TestCIRaceClassifierNonGoChangeIsEmpty(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/usr/bin/env bash\nprintf 'M\\t%s\\n' docs/README.md\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, out, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("non-Go classification failed: %v\n%s", err, out)
	}
	if len(got) != 0 {
		t.Fatalf("non-Go change selected %v", got)
	}
}

func TestCIRaceClassifierFallsBackToFullOnDiffFailure(t *testing.T) {
	got, diagnostic, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=false"}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("diff failure did not fall back: %v\n%s", err, diagnostic)
	}
	if len(got) != 61 || !strings.Contains(diagnostic, "git diff failed") {
		t.Fatalf("diff failure selected %d packages without diagnostic: %s", len(got), diagnostic)
	}
}

func TestCIRaceClassifierFallsBackToFullOnGoDeletion(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/usr/bin/env bash\nprintf 'D\\t%s\\n' internal/storage/store.go\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, diagnostic, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("deletion did not fall back: %v\n%s", err, diagnostic)
	}
	if len(got) != 61 || !strings.Contains(diagnostic, "deleted Go file") {
		t.Fatalf("deletion selected %d packages without diagnostic: %s", len(got), diagnostic)
	}
}

func TestCIRaceClassifierFallsBackToFullOnNestedModuleChange(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	if err := os.WriteFile(gitPath, []byte("#!/usr/bin/env bash\nprintf 'M\\t%s\\n' third_party/glade-apex-parser/go.mod\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, diagnostic, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("nested module change did not fall back: %v\n%s", err, diagnostic)
	}
	if len(got) != 61 {
		t.Fatalf("nested module change selected %d packages, want full 61", len(got))
	}
}

func TestCIRaceClassifierFallsBackToFullOnDependencyGraphFailure(t *testing.T) {
	dir := t.TempDir()
	gitPath := filepath.Join(dir, "git")
	goPath := filepath.Join(dir, "go")
	if err := os.WriteFile(gitPath, []byte("#!/usr/bin/env bash\nprintf 'M\\t%s\\n' internal/storage/store.go\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	goScript := `#!/usr/bin/env bash
case "$*" in
  *"./internal/storage") printf '%s\n' github.com/glade-sh/glade/internal/storage ;;
  *"./...") exit 41 ;;
  *) exit 97 ;;
esac
`
	if err := os.WriteFile(goPath, []byte(goScript), 0o700); err != nil {
		t.Fatal(err)
	}
	got, diagnostic, err := runRaceClassifier(t, []string{"CI_RACE_GIT_COMMAND=" + gitPath, "CI_RACE_GO_COMMAND=" + goPath}, "changed", "base", "head")
	if err != nil {
		t.Fatalf("graph failure did not fall back: %v\n%s", err, diagnostic)
	}
	if len(got) != 61 || !strings.Contains(diagnostic, "dependency graph failed") {
		t.Fatalf("graph failure selected %d packages without diagnostic: %s", len(got), diagnostic)
	}
}

func TestCIRaceWorkflowContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "race.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	for _, marker := range []string{
		"name: Race", "pull_request:", "branches: [main]", "schedule:", "workflow_dispatch:",
		"fetch-depth: 0", "scripts/ci-race-packages.sh", "fromJSON(needs.plan.outputs.packages)",
		"fail-fast: false", "max-parallel: 4", "GOMAXPROCS: \"2\"", "go-version: \"1.26.5\"",
		"go test -race -count=1 -timeout=60m", "scripts/ci-resource-run.sh", "if: always()",
		"actions/upload-artifact@v6", "ci-artifacts/race/", "if-no-files-found: error",
		"npm ci --prefix third_party/lwc", "contains(fromJSON('[\"./internal/gladecli\"",
	} {
		if !strings.Contains(workflow, marker) {
			t.Errorf("race workflow missing %q", marker)
		}
	}
	for _, forbidden := range []string{"pull_request_target:", "continue-on-error:", "runs-on: ubuntu-latest-", "go test -race ./..."} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("race workflow contains forbidden %q", forbidden)
		}
	}
	if strings.Contains(workflow, "Required CI") || strings.Contains(workflow, "required-ci") {
		t.Fatal("race workflow is coupled to normal Required CI")
	}
}
