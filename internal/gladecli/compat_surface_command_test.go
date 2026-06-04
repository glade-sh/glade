package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompatSurfaceRefreshWritesReports(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	out := filepath.Join(root, "out")
	if err := os.MkdirAll(filepath.Join(docs, "apex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "apex", "system_label.md"), []byte("# Label Class\n\n### get(String section, String key)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tooling := filepath.Join(root, "tooling.json")
	if err := os.WriteFile(tooling, []byte(`{"publicDeclarations":{"System":{"Label":{"methods":[{"name":"get","returnType":"String","parameters":[{"type":"String"},{"type":"String"}]}]}}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "refresh", "--docs", docs, "--tooling-completions", tooling, "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "surface refresh: ok") {
		t.Fatalf("compact summary missing: %q", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(out, "SURFACE_LEDGER.json")); err != nil {
		t.Fatalf("ledger missing: %v", err)
	}
}

func TestCompatSurfaceDryRunPrintsTempDir(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	if err := os.MkdirAll(filepath.Join(docs, "apex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "apex", "system_object.md"), []byte("# Object Class\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "refresh", "--docs", docs, "--dry-run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "dryRunOut=") {
		t.Fatalf("dry-run path missing: %q", stdout.String())
	}
}

func TestCompatSurfaceSourcesReportsCompleteDocsShelf(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	writeSurfaceSourceFixture(t, docs)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "sources", "--docs", docs}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"surface sources: ok",
		"atlas: pinned=21 missing=0 partial=0",
		"nonAtlas: lwc=present siteReferences=present",
		"files: manifest=present searchIndex=present missingLocalMarkdown=0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestCompatSurfaceSourcesCheckComparesAndPreservesBaseline(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	writeSurfaceSourceFixture(t, docs)
	checkPath := filepath.Join(root, "sources.md")

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "sources", "--docs", docs, "--output", checkPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("output exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	baseline, err := os.ReadFile(checkPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(docs, "field-reference")); err != nil {
		t.Fatal(err)
	}

	stdout.Reset()
	stderr.Reset()
	code = Run(context.Background(), []string{"compat", "surface", "sources", "--docs", docs, "--check", checkPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected failure stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "surface sources drift") {
		t.Fatalf("missing failure summary: %q", stderr.String())
	}
	data, err := os.ReadFile(checkPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(baseline) {
		t.Fatalf("check mutated baseline\nbefore:\n%s\nafter:\n%s", string(baseline), string(data))
	}
}

func TestCompatSurfaceSourcesRejectsJSONWithCheck(t *testing.T) {
	root := t.TempDir()
	docs := filepath.Join(root, "docs")
	writeSurfaceSourceFixture(t, docs)

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "sources", "--docs", docs, "--json", "--check", filepath.Join(root, "sources.md")}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected failure stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "use only one of --json, --output, or --check") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestCompatSurfacePacketWritesAreaPacket(t *testing.T) {
	root := t.TempDir()
	ledger := filepath.Join(root, "ledger.json")
	out := filepath.Join(root, "docs", "agent-packets", "salesforce", "FeatureManagement.md")
	data := `{
  "schemaVersion": 1,
  "rows": [
    {"surfaceId":"apex:System.FeatureManagement.checkPermission(String)","product":"apex","area":"runtime","namespace":"System","typeName":"FeatureManagement","memberName":"checkPermission","kind":"method","docs":"present","gladeShape":"absent","evidence":"none","gapClass":"missing-shape","priority":10},
    {"surfaceId":"apex:System.Database.executeBatch(Object,Integer)","product":"apex","area":"runtime","namespace":"System","typeName":"Database","memberName":"executeBatch","kind":"method","docs":"present","gladeShape":"absent","evidence":"none","gapClass":"missing-shape","priority":20}
  ],
  "summary": {"gaps":{"missing-shape":2},"failures":{},"total":2}
}`
	if err := os.WriteFile(ledger, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "packet", "--ledger", ledger, "--area", "Core.Runtime.System.FeatureManagement", "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d stderr=%q stdout=%q", code, stderr.String(), stdout.String())
	}
	packet, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	text := string(packet)
	for _, want := range []string{
		"# Salesforce Surface Packet: Core.Runtime.System.FeatureManagement",
		"dependsOn",
		"exclusiveFiles",
		"Area ratchet command",
		"apex:System.FeatureManagement.checkPermission(String)",
		"go test ./internal/vm ./internal/capability ./internal/repoguard",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("packet missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "executeBatch") {
		t.Fatalf("packet included a row from another area:\n%s", text)
	}
}

func TestCompatSurfacePacketRejectsUnknownArea(t *testing.T) {
	root := t.TempDir()
	ledger := filepath.Join(root, "ledger.json")
	if err := os.WriteFile(ledger, []byte(`{"schemaVersion":1,"rows":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"compat", "surface", "packet", "--ledger", ledger, "--area", "No.Such.Area"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected failure stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "unknown surface area") {
		t.Fatalf("missing unknown area error: %q", stderr.String())
	}
}

func writeSurfaceSourceFixture(t *testing.T, docs string) {
	t.Helper()
	docsets := []string{
		"apex", "apex-guide", "visualforce", "lightning", "rest-api", "tooling-api",
		"object-reference", "field-reference", "soql-sosl", "metadata-api", "soap-api",
		"bulk-api", "ui-api", "platform-events", "streaming-api", "connect-rest-api",
		"service-connector-api-reference", "limits-reference", "cli-reference",
		"analytics-cli-reference", "commerce-cli-reference", "lwc",
	}
	var manifest []string
	for _, docset := range docsets {
		dir := filepath.Join(docs, docset)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("# "+docset+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "_version.json"), []byte(`{"pages":{"success":1,"empty":0,"failed":0,"total":1}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		manifest = append(manifest, `{"path":"`+docset+`/index.md"}`)
	}
	site := filepath.Join(docs, "site-references")
	if err := os.MkdirAll(site, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "_catalog.md"), []byte("# Catalog\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "_references.json"), []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "_version.json"), []byte(`{"pages":{"success":1,"empty":0,"failed":0,"total":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(site, "index.md"), []byte("# Site references\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest = append(manifest, `{"path":"site-references/index.md"}`)
	manifest = append(manifest, `{"path":"site-references/_catalog.md"}`)
	for _, project := range []struct {
		brand string
		name  string
	}{
		{"platform", "pub-sub-api"},
		{"platform", "graphql"},
		{"ai", "agentforce"},
		{"marketing", "marketing-cloud-ampscript"},
		{"platform", "sf-connect-amazon-rds"},
	} {
		if err := os.MkdirAll(filepath.Join(site, project.brand, project.name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(docs, "manifest.json"), []byte("["+strings.Join(manifest, ",")+"]"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(docs, "search-index.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
}
