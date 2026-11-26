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
	const retryWarning = "The full CodeQL scan for this release timed out. The existing release notes may lack the disclosure."
	const summary = "Earlier step summary.\n"
	const generated = "Generated notes.\n"
	const generatedWithWarning = generated + "\n" + warning + "\n"
	const generatedWithSimilarLine = generated + "\n> " + warning + "\n"
	const existing = "Existing notes with a manual edit.\n\n"
	const warned = warning + "\n\n" + existing
	for _, tc := range []struct {
		name, status, completed, generated, body, want, mutations string
	}{
		{"timeout_new", "missing", "false", generated, "", warning + "\n\n" + generated, "create\n"},
		{"timeout_new_already_warned", "missing", "false", generatedWithWarning, "", generatedWithWarning, "create\n"},
		{"timeout_new_similar_line", "missing", "false", generatedWithSimilarLine, "", warning + "\n\n" + generatedWithSimilarLine, "create\n"},
		{"timeout_draft", "true", "false", generated, existing, existing, ""},
		{"timeout_draft_already_warned", "true", "false", generated, warned, warned, ""},
		{"completed_new", "missing", "true", generated, "", generated, "create\n"},
		{"completed_draft", "true", "true", generated, existing, existing, ""},
		{"completed_after_timeout", "true", "true", generated, warned, warned, ""},
		{"missing_output_new", "missing", "", generated, "", generated, "create\n"},
		{"missing_output_draft", "true", "", generated, existing, existing, ""},
		{"missing_output_published", "false", "", generated, existing, existing, ""},
		{"published_timeout", "false", "false", generated, existing, existing, ""},
		{"published_completed", "false", "true", generated, warned, warned, ""},
		{"publishes_before_warning_added", "publishing", "false", generated, existing, existing, ""},
		{"publishes_before_warning_removed", "publishing", "true", generated, warned, warned, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			summaryPath := filepath.Join(root, "job summary.md")
			files := map[string]string{
				"status": tc.status, "body": tc.body, "mutations": "", "release-notes.md": tc.generated,
				"job summary.md": summary,
				"git":            "#!/usr/bin/env bash\ntest \"$*\" = 'rev-list -n 1 v1.2.3' && echo test-commit\n",
				"gh":             releaseCodeQLMockGH,
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
					"GITHUB_REF_NAME=v1.2.3", "CODEQL_COMPLETED="+tc.completed, "GITHUB_STEP_SUMMARY="+summaryPath)
				// GitHub Actions gives each step attempt its own summary file.
				if err := os.WriteFile(summaryPath, []byte(summary), 0o644); err != nil {
					t.Fatal(err)
				}
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("attempt %d: %v\n%s", attempt, err, out)
				}
				wantSummary := summary
				wantAnnotations := ""
				if tc.completed == "false" && (tc.status != "missing" || attempt > 1) {
					wantSummary += "- " + retryWarning + "\n"
					wantAnnotations = "::warning::" + retryWarning
				}
				var annotations []string
				for _, line := range strings.Split(string(out), "\n") {
					if strings.HasPrefix(line, "::warning") {
						annotations = append(annotations, line)
					}
				}
				if got := strings.Join(annotations, "\n"); got != wantAnnotations {
					t.Errorf("attempt %d warning annotations = %q, want %q", attempt, got, wantAnnotations)
				}
				if got := string(mustReadFile(t, summaryPath)); got != wantSummary {
					t.Errorf("attempt %d job summary = %q, want %q", attempt, got, wantSummary)
				}
				if tc.status == "publishing" && string(mustReadFile(t, filepath.Join(root, "status"))) != "false" {
					t.Fatalf("attempt %d: mock did not publish the draft", attempt)
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
      "")
        if [[ "$status" == publishing ]]; then
          printf false > status
        fi ;;
      tagName) test "$6" = --jq && test "$7" = .tagName && echo v1.2.3 ;;
      isDraft,body)
        if [[ "$status" == publishing ]]; then
          status=true
        fi
        jq -n --argjson isDraft "$status" --rawfile body body '{isDraft:$isDraft, body:$body}' ;;
      isDraft)
        test "$6" = --jq && test "$7" = .isDraft
        if [[ "$status" == publishing ]]; then
          # A maintainer publishes after the last draft check returns its snapshot.
          status=true
          printf false > status
        fi
        printf '%s\n' "$status" ;;
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
      edit) [[ "$status" == true || "$status" == false ]] ;;
    esac
    test "$#" = 5
    test "$4" = --notes-file
    cp "$5" body
    ;;
  *) exit 2 ;;
esac
`
