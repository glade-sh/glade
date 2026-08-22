package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInspectGraphJSON(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"inspect", "graph", "--project", root, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, stderr.String())
	}
	var got struct {
		SchemaVersion string `json:"schemaVersion"`
		Command       string `json:"command"`
		Status        string `json:"status"`
		ExitCode      int    `json:"exitCode"`
		Data          struct {
			Nodes map[string]any   `json:"nodes"`
			Edges []map[string]any `json:"edges"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout is not graph JSON: %v\n%s", err, stdout.String())
	}
	if got.SchemaVersion != "1.0" || got.Command != "inspect graph" || got.Status != "passed" || got.ExitCode != 0 {
		t.Fatalf("unexpected envelope: %#v\n%s", got, stdout.String())
	}
	if len(got.Data.Nodes) == 0 {
		t.Fatalf("expected graph nodes")
	}
}

func TestRunReportAssessJSON(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "assess", "--project", root, "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, stderr.String())
	}
	var got map[string]any
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "report assess", &got)
	if env.Status != "passed" || env.ExitCode != 0 {
		t.Fatalf("unexpected report envelope: %#v\n%s", env, stdout.String())
	}
	if got["schema_version"] != "glade.enterprise.report/v0" {
		t.Fatalf("report = %#v", got)
	}
}

func TestRunReportAssessStrictFailsOnDiagnostics(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeTestFile(t, filepath.Join(root, "force-app", "main", "default", "classes", "Broken.cls"), `public class Broken {
  public void run() {
    MissingType value;
  }
}`)
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "assess", "--project", root, "--strict", "--format", "json"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit = 0 stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	var got map[string]any
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "report assess", &got)
	if env.Status != "failed" || env.ExitCode != 1 {
		t.Fatalf("unexpected report envelope: %#v\n%s", env, stdout.String())
	}
	if got["status"] != "fail" {
		t.Fatalf("status = %#v, report=%#v", got["status"], got)
	}
	if !strings.Contains(stderr.String(), "enterprise assessment failed") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunReportAcceptsUppercaseFormat(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "assess", "--project", root, "--format", "JSON"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, stderr.String())
	}
	var got map[string]any
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "report assess", &got)
	if env.Status != "passed" || got["schema_version"] != "glade.enterprise.report/v0" {
		t.Fatalf("unexpected report envelope: env=%#v data=%#v\n%s", env, got, stdout.String())
	}
}

func TestRunReportCruftWritesHTML(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	out := filepath.Join(t.TempDir(), "cruft.html")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "cruft", "--project", root, "--format", "html", "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%q", code, stderr.String())
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	if !strings.Contains(string(data), "Glade Enterprise Report") {
		t.Fatalf("html = %s", string(data))
	}
	if !strings.Contains(stdout.String(), out) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunReportRefactorProofJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "refactor-proof", "--project", ".", "--since", "HEAD", "--format", "json"}, &stdout, &stderr)
	var got map[string]any
	env := decodeCLIEnvelopeData(t, stdout.Bytes(), "report refactor-proof", &got)
	if env.ExitCode != code {
		t.Fatalf("envelope exitCode=%d process=%d\n%s", env.ExitCode, code, stdout.String())
	}
	if got["schema_version"] != "glade.enterprise.report/v0" {
		t.Fatalf("report = %#v", got)
	}
	if got["status"] == "fail" && code == 0 {
		t.Fatalf("exit = 0 for failed proof report: %#v", got)
	}
	if got["status"] != "fail" && code != 0 {
		t.Fatalf("exit = %d stderr=%q report=%#v", code, stderr.String(), got)
	}
}

func TestRunReportRefactorProofRejectsRunTestsFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "refactor-proof", "--run-tests"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit = 0 stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown flag "--run-tests"`) {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestRunReportInvalidFormatDoesNotCreateOutFile(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "local-tests", "enterprise-composed")
	out := filepath.Join(t.TempDir(), "report.bad")
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"report", "assess", "--project", root, "--format", "bad", "--out", out}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit = 0 stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "--format must be json, html, or md") {
		t.Fatalf("stderr = %q", stderr.String())
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("out file exists or stat failed unexpectedly: %v", err)
	}
}

func TestRunReportRejectsFlagsForWrongEnterpriseSubcommand(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "assess fail on api break",
			args: []string{"report", "assess", "--fail-on-api-break"},
			want: "glade report assess only accepts",
		},
		{
			name: "cruft include tests",
			args: []string{"report", "cruft", "--include-tests"},
			want: "glade report cruft only accepts",
		},
		{
			name: "refactor proof strict",
			args: []string{"report", "refactor-proof", "--strict"},
			want: "glade report refactor-proof only accepts",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), tc.args, &stdout, &stderr)
			if code == 0 {
				t.Fatalf("exit = 0 stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.want)
			}
		})
	}
}
