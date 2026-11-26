package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestReleaseCodeQLCompletionOutput(t *testing.T) {
	var codeql struct {
		On struct {
			WorkflowCall struct {
				Outputs map[string]struct {
					Value string `yaml:"value"`
				} `yaml:"outputs"`
			} `yaml:"workflow_call"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Outputs map[string]string `yaml:"outputs"`
			Steps   []struct {
				ID   string `yaml:"id"`
				Name string `yaml:"name"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(readWorkflowFile(t, "codeql.yml")), &codeql); err != nil {
		t.Fatalf("parse codeql.yml: %v", err)
	}
	job := codeql.Jobs["codeql"]
	analyzeID := ""
	for _, step := range job.Steps {
		if step.Name == "Analyze CodeQL" {
			analyzeID = step.ID
		}
	}
	if got := job.Outputs["completed"]; analyzeID == "" || got != "${{ steps."+analyzeID+".outputs.completed }}" {
		t.Errorf("CodeQL job completed output = %q, want completion from the Analyze CodeQL step", got)
	}
	if got := codeql.On.WorkflowCall.Outputs["completed"].Value; got != "${{ jobs.codeql.outputs.completed }}" {
		t.Errorf("CodeQL workflow completed output = %q, want completion from the codeql job", got)
	}

	workflow, source := readReleaseWorkflowDocument(t)
	prepare := workflow.Jobs["prepare"]
	if !reflect.DeepEqual(prepare.Needs, []any{"required-ci-attestation", "salesforce-authority", "codeql-release"}) ||
		!strings.Contains(prepare.If, "needs.codeql-release.result == 'success'") {
		t.Errorf("release preparation must still require successful CodeQL: needs=%v if=%q", prepare.Needs, prepare.If)
	}
	create := prepare.Steps[releaseWorkflowStepIndex(t, prepare, "Create GitHub Release")]
	if got := create.Env["CODEQL_COMPLETED"]; got != "${{ needs.codeql-release.outputs.completed }}" {
		t.Errorf("release draft step CODEQL_COMPLETED = %q, want completion from codeql-release", got)
	}
	jobs := workflowJobBlocksText(source)
	for _, name := range []string{"prepare", "publish"} {
		if !strings.Contains(jobs[name], "\n    concurrency: release-metadata-${{ github.ref }}\n") {
			t.Errorf("%s must serialize release metadata changes for the tag", name)
		}
	}
}

func TestReleaseCodeQLDraftNotes(t *testing.T) {
	workflow, _ := readReleaseWorkflowDocument(t)
	prepare := workflow.Jobs["prepare"]
	create := prepare.Steps[releaseWorkflowStepIndex(t, prepare, "Create GitHub Release")]
	const warning = "Note: the full CodeQL scan for this release did not complete within its time limit."
	const generated = "Generated notes.\n"
	const existing = "Existing notes with a manual edit.\n\n"
	const warned = warning + "\n\n" + existing
	for _, tc := range []struct {
		name, status, completed, body, want, mutations string
	}{
		{"timeout_new", "missing", "false", "", warning + "\n\n" + generated, "create\n"},
		{"timeout_draft", "true", "false", existing, warned, "edit\n"},
		{"completed_new", "missing", "true", "", generated, "create\n"},
		{"completed_draft", "true", "true", existing, existing, ""},
		{"completed_after_timeout", "true", "true", warned, existing, "edit\n"},
		{"missing_output_draft", "true", "", existing, existing, ""},
		{"published_timeout", "false", "false", existing, existing, ""},
		{"published_completed", "false", "true", warned, warned, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"status": tc.status, "body": tc.body, "mutations": "", "release-notes.md": generated,
				"git": "#!/usr/bin/env bash\ntest \"$*\" = 'rev-list -n 1 v1.2.3' && echo test-commit\n",
				"gh":  releaseCodeQLMockGH,
			}
			for name, content := range files {
				if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for attempt := 1; attempt <= 2; attempt++ {
				cmd := exec.Command("bash", "-e", "-c", create.Run)
				cmd.Dir = root
				cmd.Env = append(os.Environ(), "PATH="+root+string(os.PathListSeparator)+os.Getenv("PATH"),
					"GITHUB_REF_NAME=v1.2.3", "CODEQL_COMPLETED="+tc.completed)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("attempt %d: %v\n%s", attempt, err, out)
				}
				for file, want := range map[string]string{"body": tc.want, "mutations": tc.mutations} {
					if got := string(mustReadFile(t, filepath.Join(root, file))); got != want {
						t.Errorf("attempt %d %s = %q, want %q", attempt, file, got, want)
					}
				}
			}
		})
	}
}

const releaseCodeQLMockGH = `#!/usr/bin/env bash
set -eu
test "$1" = release
test "$3" = v1.2.3
status="$(cat status)"
case "$2" in
  view)
    [[ "$status" != missing ]] || exit 1
    case "${5:-}" in
      "") ;;
      tagName) test "$6" = --jq && test "$7" = .tagName && echo v1.2.3 ;;
      isDraft,body) jq -n --argjson isDraft "$status" --rawfile body body '{isDraft:$isDraft, body:$body}' ;;
      *) exit 2 ;;
    esac ;;
  create|edit)
    printf '%s\n' "$2" >> mutations
    case "$2" in
      create)
        test "$status" = missing
        test "$4 $5 $6 $7 $8" = '--target test-commit --draft --title v1.2.3'
        shift 5
        printf true > status ;;
      edit) test "$status" = true ;;
    esac
    test "$#" = 5
    test "$4" = --notes-file
    cp "$5" body
    ;;
  *) exit 2 ;;
esac
`
