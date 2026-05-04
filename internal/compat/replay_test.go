package compat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReplayBundleValidatesSchemaAndPaths(t *testing.T) {
	root := t.TempDir()
	writeCompatTestFile(t, filepath.Join(root, "replay.json"), `{
  "schemaVersion": 1,
  "name": "escape",
  "steps": [{"name": "check", "kind": "check", "expect": "../expected.json"}]
}`)
	if _, err := LoadReplayBundle(root); err == nil || !strings.Contains(err.Error(), "must stay inside bundle root") {
		t.Fatalf("LoadReplayBundle err = %v", err)
	}
}

func TestRunReplayBundleCheckSuccess(t *testing.T) {
	root := writeReplayCheckBundle(t, "check-success", "public class Hello {}", `{"diagnostics":0,"files":1,"ok":true,"packageDirectories":1,"types":1}`)
	report, err := RunReplayBundle(root, ReplayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Passed != 1 || len(report.Steps) != 1 {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunReplayBundleRejectsFailingCheckStep(t *testing.T) {
	root := writeReplayCheckBundle(t, "check-fail", "public class Hello { MissingType value; }", "")
	report, err := RunReplayBundle(root, ReplayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.Failed != 1 {
		t.Fatalf("report = %#v", report)
	}
	if len(report.Steps[0].Diagnostics) == 0 {
		t.Fatalf("step diagnostics missing: %#v", report.Steps[0])
	}
}

func TestRunReplayBundleClassifiesUnsupportedExec(t *testing.T) {
	root := t.TempDir()
	writeCompatTestFile(t, filepath.Join(root, "replay.json"), `{
  "schemaVersion": 1,
  "name": "unsupported-exec",
  "steps": [{"name": "exec", "kind": "exec", "args": ["System.nope();"]}]
}`)
	report, err := RunReplayBundle(root, ReplayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.Unsupported != 1 {
		t.Fatalf("report = %#v", report)
	}
	if got := report.Steps[0].Blockers[0].Category; got != "stdlib" {
		t.Fatalf("category = %q", got)
	}
}

func TestRunReplayBundleServerSequence(t *testing.T) {
	root := t.TempDir()
	writeCompatTestFile(t, filepath.Join(root, "replay.json"), `{
  "schemaVersion": 1,
  "name": "server-sequence",
  "steps": [{
    "name": "versions",
    "kind": "server",
    "serverRequests": [{"method": "GET", "path": "/services/data/", "status": 200, "contains": ["v65.0"]}]
  }]
}`)
	report, err := RunReplayBundle(root, ReplayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Summary.Passed != 1 {
		t.Fatalf("report = %#v", report)
	}
}

func TestRunCheckedInReplayFixtures(t *testing.T) {
	for _, path := range []string{
		"../../testdata/replay/selector-service-domain",
		"../../testdata/replay/server-backed",
	} {
		report, err := RunReplayBundle(path, ReplayOptions{})
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !report.OK {
			t.Fatalf("%s report = %#v", path, report)
		}
	}
}

func TestReplayArtifactsRedactHeaders(t *testing.T) {
	root := writeReplayCheckBundle(t, "artifact-check", "public class Hello {}", "")
	writeCompatTestFile(t, filepath.Join(root, "fixtures", "headers.json"), `{"headers":{"Authorization":"Bearer secret","x-api-token":"abc","safe":"ok"}}`)
	artifacts := filepath.Join(t.TempDir(), "artifacts")
	report, err := RunReplayBundle(root, ReplayOptions{ArtifactsDir: artifacts})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK {
		t.Fatalf("report = %#v", report)
	}
	copied, err := os.ReadFile(filepath.Join(artifacts, "bundle", "fixtures", "headers.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(copied)
	if strings.Contains(text, "Bearer secret") || strings.Contains(text, "abc") || !strings.Contains(text, `"safe": "ok"`) {
		t.Fatalf("redacted artifact = %s", text)
	}
}

func TestReplayArtifactsRejectSymlink(t *testing.T) {
	root := writeReplayCheckBundle(t, "artifact-symlink", "public class Hello {}", "")
	secret := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(secret, []byte("do not copy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secret, filepath.Join(root, "secret-link.txt")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := RunReplayBundle(root, ReplayOptions{ArtifactsDir: filepath.Join(t.TempDir(), "artifacts")})
	if err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("RunReplayBundle err = %v", err)
	}
}

func TestAnalyzeReadinessReportsSemaBlocker(t *testing.T) {
	root := t.TempDir()
	writeCompatTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeCompatTestFile(t, filepath.Join(root, "force-app/main/default/classes/Hello.cls"), "public class Hello { MissingType value; }")
	report, err := AnalyzeReadiness(root)
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Summary.Categories["sema"] != 1 {
		t.Fatalf("report = %#v", report)
	}
	if report.Blockers[0].Symbol != "MissingType" {
		t.Fatalf("blocker = %#v", report.Blockers[0])
	}
}

func TestWriteReplayJSONSchema(t *testing.T) {
	root := writeReplayCheckBundle(t, "json-check", "public class Hello {}", "")
	report, err := RunReplayBundle(root, ReplayOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var encoded strings.Builder
	if err := WriteReplayJSON(&encoded, report); err != nil {
		t.Fatal(err)
	}
	var decoded ReplayReport
	if err := json.Unmarshal([]byte(encoded.String()), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != 1 || decoded.Name != "json-check" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func writeReplayCheckBundle(t *testing.T, name, source, expect string) string {
	t.Helper()
	root := t.TempDir()
	writeCompatTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
	writeCompatTestFile(t, filepath.Join(root, "force-app/main/default/classes/Hello.cls"), source)
	expectLine := ""
	if expect != "" {
		writeCompatTestFile(t, filepath.Join(root, "expected", "check.json"), expect)
		expectLine = `, "expect": "expected/check.json"`
	}
	writeCompatTestFile(t, filepath.Join(root, "replay.json"), `{
  "schemaVersion": 1,
  "name": "`+name+`",
  "project": {"root": "."},
  "steps": [{"name": "check", "kind": "check"`+expectLine+`}]
}`)
	return root
}

func writeCompatTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
