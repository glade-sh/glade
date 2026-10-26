package scripts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCIWorkflowLintLaneCommandRoute(t *testing.T) {
	for _, status := range []int{0, 23} {
		t.Run(fmt.Sprintf("test_exit_%d", status), func(t *testing.T) {
			root := t.TempDir()
			calls := filepath.Join(root, "calls")
			events := "{\"Action\":\"output\",\"Output\":\"workflow lint fixture\\n\"}\n"
			actionlintFile(t, filepath.Join(root, "go"), "#!/bin/bash\nprintf '%s\\n' \"$*\" >\"$FIXTURE_CALLS\"\nprintf '%s' \"$FIXTURE_EVENTS\"\nexit \"$FIXTURE_STATUS\"\n")
			actionlintFile(t, filepath.Join(root, "testlog"), "#!/bin/bash\ncat >/dev/null\n")
			artifacts := filepath.Join(root, "artifacts")
			cmd := exec.Command("bash", "ci-go-test.sh", "lane", "workflow-lint")
			cmd.Env = append(os.Environ(), "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"),
				"CI_TESTLOG_RENDERER="+filepath.Join(root, "testlog"), "CI_TEST_BIN_DIR=",
				"CI_GO_TEST_ARTIFACT_DIR="+artifacts, "FIXTURE_CALLS="+calls,
				"FIXTURE_EVENTS="+events, fmt.Sprintf("FIXTURE_STATUS=%d", status))
			out, err := cmd.CombinedOutput()
			gotStatus := 0
			if err != nil {
				if exit, ok := err.(*exec.ExitError); ok {
					gotStatus = exit.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if gotStatus != status {
				t.Fatalf("lane exit = %d, want %d\n%s", gotStatus, status, out)
			}
			want := "test -json -vet=off -trimpath -count=1 -timeout=3m -run ^TestCIActionlint ./scripts/ci_actionlint_test.go\n"
			if got := actionlintRead(t, calls); got != want {
				t.Fatalf("lane command = %q, want %q", got, want)
			}
			if got := actionlintRead(t, filepath.Join(artifacts, "test-workflow-lint.json")); got != events {
				t.Fatalf("raw events = %q, want %q", got, events)
			}
		})
	}
}

func TestCIWorkflowLintLaneSyntheticProjectRoute(t *testing.T) {
	requireCIActionlint(t)
	for _, invalid := range []bool{false, true} {
		t.Run(fmt.Sprintf("invalid_workflow_%t", invalid), func(t *testing.T) {
			root := t.TempDir()
			// Recreate a project with the real workflow inventory and offline tests.
			// Invoke the CI lane rather than calling actionlint directly.
			files := []string{"ci-go-test.sh", "ci-actionlint.sh", "install-actionlint.sh", "ci_actionlint_test.go",
				"testdata/actionlint/invalid-job-env.yml", "testdata/actionlint/valid-step-env.yml"}
			for _, file := range files {
				actionlintFile(t, filepath.Join(root, "scripts", file), actionlintRead(t, file))
			}
			workflows, err := filepath.Glob("../.github/workflows/*")
			if err != nil || len(workflows) == 0 {
				t.Fatalf("workflow inventory: %v, files=%v", err, workflows)
			}
			for _, file := range workflows {
				actionlintFile(t, filepath.Join(root, ".github/workflows", filepath.Base(file)), actionlintRead(t, file))
			}
			if invalid {
				actionlintFile(t, filepath.Join(root, ".github/workflows/invalid.yml"), actionlintRead(t, "testdata/actionlint/invalid-job-env.yml"))
			}
			renderer := filepath.Join(root, "testlog")
			actionlintFile(t, renderer, "#!/bin/bash\ncat >/dev/null\n")
			artifacts := filepath.Join(root, "artifacts")
			cmd := exec.Command("bash", "scripts/ci-go-test.sh", "lane", "workflow-lint")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "CI_TESTLOG_RENDERER="+renderer, "CI_TEST_BIN_DIR=", "CI_GO_TEST_ARTIFACT_DIR="+artifacts)
			out, err := cmd.CombinedOutput()
			if (err != nil) != invalid {
				t.Fatalf("lane error = %v, invalid = %t\n%s", err, invalid, out)
			}
			events := actionlintRead(t, filepath.Join(artifacts, "test-workflow-lint.json"))
			if !strings.Contains(events, `"Test":"TestCIActionlintRealRepositoryWorkflows"`) {
				t.Fatalf("lane did not execute the real repository route:\n%s", events)
			}
			if invalid && !strings.Contains(events, `context \"runner\" is not allowed here`) {
				t.Fatalf("invalid project did not report the broken job-env route:\n%s", events)
			}
			if strings.Contains(events, `"Action":"skip"`) {
				t.Fatalf("installed actionlint checks must execute:\n%s", events)
			}
		})
	}
}
