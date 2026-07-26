package sema

import (
	"os"
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

func TestSecurityEnforcedQueryRemainsAcceptedAtAPIVersion67AndLater(t *testing.T) {
	source := `
public class Probe {
  public static void run() {
    List<Account> rows = [SELECT Id FROM Account WITH SECURITY_ENFORCED];
  }
}
`
	for _, test := range []struct {
		apiVersion string
	}{
		{apiVersion: "66.0"},
		{apiVersion: "67.0"},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{SourceAPIVersion: test.apiVersion}, queryDiagnosticSchema())
			if hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
				t.Fatalf("API %s query diagnostics = %#v, want acceptance", test.apiVersion, result.Diagnostics)
			}
		})
	}
}

func TestPropertyGetterMutationIsRejectedFromAPIVersion42(t *testing.T) {
	source := `public class Probe { public Integer Value { get { Value = 1; return Value; } set; } }`
	for _, test := range []struct {
		apiVersion string
		wantError  bool
	}{
		{apiVersion: "41.0", wantError: false},
		{apiVersion: "42.0", wantError: true},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, test.apiVersion)
			if result.HasErrors() != test.wantError {
				t.Fatalf("API %s diagnostics = %#v, want error=%v", test.apiVersion, result.Diagnostics, test.wantError)
			}
		})
	}
}

func TestAbstractMethodRequiresExplicitAccessAtAPIVersion65(t *testing.T) {
	source := `public abstract class Probe { abstract String value(); }`
	for _, test := range []struct {
		apiVersion string
		wantReject bool
	}{
		{apiVersion: "64.0", wantReject: false},
		{apiVersion: "65.0", wantReject: true},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, test.apiVersion)
			gotReject := hasDiagnosticCode(result.Diagnostics, "GLADESEMA032")
			if gotReject != test.wantReject {
				t.Fatalf("API %s diagnostics = %#v, want reject=%v", test.apiVersion, result.Diagnostics, test.wantReject)
			}
		})
	}
}

func TestAbstractMethodVersionGateUsesEffectiveSourceAPIVersion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Probe.cls")
	writeSemaFile(t, path, `public abstract class Probe { abstract String value(); }`)
	if err := os.WriteFile(filepath.Join(root, "Probe.cls-meta.xml"), []byte("<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>65.0</apiVersion></ApexClass>"), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	result := Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: "64.0", ApexFiles: []string{path}}, schema.Schema{}))
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
		t.Fatalf("effective API 65 source was accepted: %#v", result.Diagnostics)
	}
}
