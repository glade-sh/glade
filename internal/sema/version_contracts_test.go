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

func TestLegacySiteURLHelpersUseEffectiveSourceAPIVersion(t *testing.T) {
	source := `public class Probe { public void run() { String value = Site.getCurrentSiteUrl(); } }`
	for _, test := range []struct {
		apiVersion string
		wantError  bool
	}{
		{apiVersion: "29.0", wantError: false},
		{apiVersion: "30.0", wantError: true},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, test.apiVersion)
			gotError := hasDiagnosticCode(result.Diagnostics, "GLADESEMA028")
			if gotError != test.wantError {
				t.Fatalf("API %s Site.getCurrentSiteUrl error = %v, want %v: %#v", test.apiVersion, gotError, test.wantError, result.Diagnostics)
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
				t.Fatalf("API %s query rejection = %v diagnostics=%#v", test.apiVersion, gotReject, result.Diagnostics)
			}
		})
	}
}

func TestSecurityEnforcedQueryUsesEffectiveSourceAPIVersion(t *testing.T) {
	source := `
public class Probe {
  public static void run() {
    List<Account> rows = [SELECT Id FROM Account WITH SECURITY_ENFORCED];
  }
}
`
	for _, test := range []struct {
		projectVersion string
		sourceVersion  string
		wantReject     bool
	}{
		{projectVersion: "66.0", sourceVersion: "67.0", wantReject: true},
		{projectVersion: "67.0", sourceVersion: "66.0", wantReject: false},
	} {
		t.Run(test.projectVersion+"_source_"+test.sourceVersion, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "Probe.cls")
			writeSemaFile(t, path, source)
			metadata := "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>" + test.sourceVersion + "</apiVersion></ApexClass>"
			if err := os.WriteFile(path+"-meta.xml", []byte(metadata), 0o600); err != nil {
				t.Fatalf("write metadata: %v", err)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: test.projectVersion, ApexFiles: []string{path}}, queryDiagnosticSchema())
			result := Analyze(index)
			gotReject := hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT")
			if gotReject != test.wantReject {
				t.Fatalf("project API %s source API %s query rejection = %v diagnostics=%#v", test.projectVersion, test.sourceVersion, gotReject, result.Diagnostics)
			}
		})
	}
}

func TestReadOnlyRESTMethodRemoteActionVersionGate(t *testing.T) {
	source := `@RestResource(urlMapping='/probe') global class Probe { @ReadOnly @HttpGet global static void run() {} }`
	for _, test := range []struct {
		apiVersion string
		wantReject bool
	}{
		{apiVersion: "48.0", wantReject: true},
		{apiVersion: "49.0", wantReject: false},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, test.apiVersion)
			gotReject := hasDiagnosticCode(result.Diagnostics, "GLADESEMA032")
			if gotReject != test.wantReject {
				t.Fatalf("API %s ReadOnly REST rejection = %v diagnostics=%#v", test.apiVersion, gotReject, result.Diagnostics)
			}
		})
	}
}

func TestAuraEnabledGlobalScopeVersionGate(t *testing.T) {
	source := `public class Probe { @AuraEnabled(cacheable=true scope='global') public static String run() { return 'ok'; } }`
	for _, test := range []struct {
		apiVersion string
		wantReject bool
	}{
		{apiVersion: "54.0", wantReject: true},
		{apiVersion: "55.0", wantReject: false},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, test.apiVersion)
			gotReject := hasDiagnosticCode(result.Diagnostics, "GLADESEMA032")
			if gotReject != test.wantReject {
				t.Fatalf("API %s AuraEnabled scope rejection = %v diagnostics=%#v", test.apiVersion, gotReject, result.Diagnostics)
			}
		})
	}
}

func TestPropertyGetterMutationRemainsAcceptedAtTestedAPIVersions(t *testing.T) {
	source := `public class Probe { public Integer Value { get { Value = 1; return Value; } set; } }`
	for _, apiVersion := range []string{"41.0", "42.0"} {
		t.Run(apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, apiVersion)
			if hasErrorOtherThan(result.Diagnostics, "GLADESEMA_VERSION") {
				t.Fatalf("API %s getter mutation control was rejected: %#v", apiVersion, result.Diagnostics)
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

func TestInvocableParameterConstructorVisibilityFollowsAPIVersion(t *testing.T) {
	sources := map[string]string{
		"Payload.cls": `public class Payload { private Payload() {} }`,
		"Action.cls": `public class Action {
  @InvocableMethod public static List<Payload> run(List<Payload> values) { return values; }
}`,
	}
	if result := analyzeDeclarationProjectWithAPIVersion(t, sources, "65.0"); result.HasErrors() {
		t.Fatalf("API 65 diagnostics = %#v", result.Diagnostics)
	}
	result := analyzeDeclarationProjectWithAPIVersion(t, sources, "66.0")
	if !result.HasErrors() || !resultDiagnosticsContain(result, "visible no-argument constructor") {
		t.Fatalf("API 66 diagnostics = %#v", result.Diagnostics)
	}
	sources["Payload.cls"] = `public class Payload { public Payload() {} }`
	if result := analyzeDeclarationProjectWithAPIVersion(t, sources, "66.0"); result.HasErrors() {
		t.Fatalf("API 66 visible-constructor diagnostics = %#v", result.Diagnostics)
	}
}
