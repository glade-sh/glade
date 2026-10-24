package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireCIActionlint(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("actionlint"); err != nil {
		t.Skip("actionlint is absent from PATH; install the documented pin to run offline workflow validation (CI installs it)")
	}
}

func actionlintFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o755); err != nil {
		t.Fatal(err)
	}
}

func actionlintRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestCIActionlintExplicitWorkflowRoute(t *testing.T) {
	requireCIActionlint(t)
	for _, tc := range []struct {
		name, fixture string
		invalid       bool
	}{
		{"job_env_runner_context_rejected", "invalid-job-env.yml", true},
		{"step_env_runner_context_allowed", "valid-step-env.yml", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := exec.Command("bash", "ci-actionlint.sh", filepath.Join("testdata", "actionlint", tc.fixture)).CombinedOutput()
			if tc.invalid {
				if err == nil || !strings.Contains(string(out), `context "runner" is not allowed here`) {
					t.Fatalf("invalid job env must fail with the context diagnostic: %v\n%s", err, out)
				}
			} else if err != nil {
				t.Fatalf("valid step env rejected: %v\n%s", err, out)
			}
		})
	}
}

func TestCIActionlintRepositoryDiscoveryRoute(t *testing.T) {
	requireCIActionlint(t)
	wrapper := actionlintRead(t, "ci-actionlint.sh")
	for _, route := range []string{"valid_workflows", "invalid_yml", "invalid_yaml", "empty_repository"} {
		t.Run(route, func(t *testing.T) {
			root := t.TempDir()
			script := filepath.Join(root, "scripts", "ci-actionlint.sh")
			actionlintFile(t, script, wrapper)
			if route != "empty_repository" {
				actionlintFile(t, filepath.Join(root, ".github", "workflows", "valid.yml"), actionlintRead(t, "testdata/actionlint/valid-step-env.yml"))
			}
			if strings.HasPrefix(route, "invalid_") {
				ext := strings.TrimPrefix(route, "invalid_")
				actionlintFile(t, filepath.Join(root, ".github", "workflows", "invalid."+ext), actionlintRead(t, "testdata/actionlint/invalid-job-env.yml"))
			}
			// Invoke from outside the project, just as a caller using an absolute script path can.
			cmd := exec.Command("bash", script)
			cmd.Dir = t.TempDir()
			out, err := cmd.CombinedOutput()
			switch {
			case route == "valid_workflows":
				if err != nil {
					t.Fatalf("discovered valid workflow rejected: %v\n%s", err, out)
				}
			case route == "empty_repository":
				if err == nil || !strings.Contains(string(out), "no workflow files") {
					t.Fatalf("empty discovery must fail explicitly: %v\n%s", err, out)
				}
			default:
				if err == nil || !strings.Contains(string(out), `context "runner" is not allowed here`) {
					t.Fatalf("discovered invalid workflow accepted: %v\n%s", err, out)
				}
			}
		})
	}
}

func TestCIActionlintRealRepositoryWorkflows(t *testing.T) {
	requireCIActionlint(t)
	out, err := exec.Command("bash", "ci-actionlint.sh").CombinedOutput()
	if err != nil {
		t.Fatalf("repository workflows rejected: %v\n%s", err, out)
	}
}

func TestCIActionlintMissingToolRoute(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bash, "ci-actionlint.sh")
	cmd.Env = append(os.Environ(), "PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "actionlint is required") {
		t.Fatalf("missing actionlint must fail explicitly: %v\n%s", err, out)
	}
}

func TestCIActionlintInstallerRejectsCorruptArchiveBeforeExtraction(t *testing.T) {
	root := t.TempDir()
	// Stub the network boundary. The real checksum utility must reject these bytes;
	// neither tar nor any downloaded executable may run.
	actionlintFile(t, filepath.Join(root, "curl"), "#!/bin/bash\nwhile [[ $# -gt 0 ]]; do\n  if [[ $1 == -o ]]; then printf 'corrupt archive' >\"$2\"; exit 0; fi\n  shift\ndone\nexit 2\n")
	marker := filepath.Join(root, "extracted")
	actionlintFile(t, filepath.Join(root, "tar"), "#!/bin/bash\ntouch \"$ACTIONLINT_TEST_MARKER\"\nexit 0\n")
	cmd := exec.Command("bash", "install-actionlint.sh", filepath.Join(root, "install"))
	cmd.Env = append(os.Environ(), "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"), "ACTIONLINT_TEST_MARKER="+marker)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "checksum") {
		t.Fatalf("corrupt download must fail checksum verification: %v\n%s", err, out)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("archive extraction ran before checksum validation: %v", err)
	}
}

func TestCIActionlintCIJobInstallsAndRunsOfflineChecks(t *testing.T) {
	workflow := actionlintRead(t, "../.github/workflows/ci.yml")
	_, job, ok := strings.Cut(workflow, "\n  workflow-lint:\n")
	if !ok {
		t.Fatal("CI lacks the workflow-lint job")
	}
	// Only inspect this job, not steps or permissions in unrelated jobs.
	for _, line := range strings.Split(job, "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.TrimSpace(line) != "" {
			job, _, _ = strings.Cut(job, "\n"+line)
			break
		}
	}
	for _, want := range []string{
		"runs-on: ubuntu-latest", "contents: read", "persist-credentials: false",
		"scripts/install-actionlint.sh", "$GITHUB_PATH",
		"scripts/ci-resource-run.sh ci-artifacts/go-test/resource-workflow-lint.json workflow-lint -- scripts/ci-go-test.sh lane workflow-lint",
		"name: go-test-workflow-lint", "if: always()", "if-no-files-found: error",
		"ci-artifacts/go-test/test-workflow-lint.json",
		"run: scripts/ci-actionlint.sh",
	} {
		if !strings.Contains(job, want) {
			t.Errorf("workflow-lint job lacks %q", want)
		}
	}
	if strings.Contains(job, "continue-on-error:") {
		t.Fatal("workflow validation must fail CI on errors")
	}
	previous := -1
	for _, step := range []string{"scripts/install-actionlint.sh", "scripts/ci-go-test.sh lane workflow-lint", "run: scripts/ci-actionlint.sh", "name: Upload raw workflow validation test events"} {
		index := strings.Index(job, step)
		if index <= previous {
			t.Errorf("workflow-lint step %q must follow its prerequisites", step)
		}
		previous = index
	}
}
