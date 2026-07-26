package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func analyzeDeclarationProjectWithAPIVersion(t *testing.T, files map[string]string, apiVersion string) Result {
	t.Helper()
	root := t.TempDir()
	paths := make([]string, 0, len(files))
	for name, contents := range files {
		path := filepath.Join(root, name)
		writeSemaFile(t, path, contents)
		paths = append(paths, path)
	}
	return Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: apiVersion, ApexFiles: paths}, schema.Schema{}))
}

func TestPreviewAnnotationsRemainDisabledAtLatestAPIVersion(t *testing.T) {
	for name, source := range map[string]string{
		"IntegrationTest": `@IntegrationTest public class Probe {}`,
		"TearDown":        `@IsTest private class Probe { @TearDown static void clean() {} }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, "67.0")
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA031") {
				t.Fatalf("preview annotation was accepted at API 67: %#v", result.Diagnostics)
			}
		})
	}
}

func TestSecurityEnforcedQueryIsRejectedAtAPIVersion67AndLater(t *testing.T) {
	source := `
public class Probe {
  public static void run() {
    List<Account> rows = [SELECT Id FROM Account WITH SECURITY_ENFORCED];
  }
}
`
	for _, test := range []struct {
		apiVersion string
		wantReject bool
	}{
		{apiVersion: "66.0", wantReject: false},
		{apiVersion: "67.0", wantReject: true},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{SourceAPIVersion: test.apiVersion}, queryDiagnosticSchema())
			gotReject := hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT")
			if gotReject != test.wantReject {
				t.Fatalf("API %s query diagnostics = %#v, want reject=%v", test.apiVersion, result.Diagnostics, test.wantReject)
			}
		})
	}
}
